package boot

import (
	"context"
	"fmt"
	"net"

	"github.com/helthtech/core-health/internal/labimport"
	"github.com/helthtech/core-health/internal/middleware"
	"github.com/helthtech/core-health/internal/migration"
	"github.com/helthtech/core-health/internal/obs"
	"github.com/helthtech/core-health/internal/repository"
	"github.com/helthtech/core-health/internal/server"
	"github.com/helthtech/core-health/internal/service"
	pb "github.com/helthtech/core-health/pkg/proto/health"
	"github.com/nats-io/nats.go"
	"github.com/porebric/configs"
	criteriapb "github.com/porebric/creteria_parser/pkg/proto/criteria"
	"github.com/porebric/logger"
	"github.com/porebric/resty"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gorml "gorm.io/gorm/logger"
)

func Run(ctx context.Context) error {
	db, err := initDB(ctx)
	if err != nil {
		return fmt.Errorf("init db: %w", err)
	}
	if err = migration.Run(db); err != nil {
		return fmt.Errorf("migration: %w", err)
	}

	nc, err := initNATS(ctx)
	if err != nil {
		return fmt.Errorf("init nats: %w", err)
	}

	rdb, err := initRedis(ctx)
	if err != nil {
		return fmt.Errorf("init redis: %w", err)
	}

	tp, err := initTracer(ctx)
	if err != nil {
		obs.BG("tracer").Error(err, "tracer init failed (non-fatal)")
	} else {
		otel.SetTracerProvider(tp)
		defer func() { _ = tp.Shutdown(context.Background()) }()
	}

	repo := repository.NewHealthRepository(db)
	svc := service.NewHealthService(repo, nc, rdb)

	var parserClient criteriapb.CriteriaParserClient
	var parserConn *grpc.ClientConn
	if addr := configs.Value(ctx, "grpc_creteria_parser").String(); addr != "" {
		var err error
		parserConn, err = grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return fmt.Errorf("grpc creteria_parser: %w", err)
		}
		parserClient = criteriapb.NewCriteriaParserClient(parserConn)
	}
	labStore := labimport.NewStore(rdb)

	// Start in-memory cache refresh loop.
	svc.StartCache(ctx)

	// Start schedulers in background.
	go svc.RunDailyScheduler(ctx, []string{"telegram", "max"})
	go svc.RunExpiryScheduler(ctx, []string{"telegram", "max"})
	go svc.RunAlarmScheduler(ctx, []string{"telegram", "max"})
	go svc.RunWeeklyScheduler(ctx)

	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ChainUnaryInterceptor(middleware.GRPCUnaryAccessLog()),
		grpc.ChainStreamInterceptor(middleware.GRPCStreamAccessLog()),
	)
	pb.RegisterHealthServiceServer(grpcServer, server.NewHealthServer(svc, parserClient, labStore))

	grpcPort := configs.Value(ctx, "grpc_port").String()
	lis, err := net.Listen("tcp", "0.0.0.0:"+grpcPort)
	if err != nil {
		return fmt.Errorf("listen grpc: %w", err)
	}

	go func() {
		obs.L.Info("gRPC server listening", "addr", grpcPort)
		if err := grpcServer.Serve(lis); err != nil {
			obs.L.Error(err, "gRPC serve error")
		}
	}()

	router := resty.NewRouter(func() *logger.Logger { return obs.L }, nil)
	resty.RunServer(ctx, router, func(ctx context.Context) error {
		grpcServer.GracefulStop()
		if parserConn != nil {
			_ = parserConn.Close()
		}
		nc.Close()
		return nil
	})

	return nil
}

func initDB(ctx context.Context) (*gorm.DB, error) {
	dsn := configs.Value(ctx, "db_dsn").String()
	return gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: gorml.Default.LogMode(gorml.Warn),
	})
}

func initNATS(ctx context.Context) (*nats.Conn, error) {
	return nats.Connect(configs.Value(ctx, "nats_url").String())
}

func initRedis(ctx context.Context) (*redis.Client, error) {
	addr := configs.Value(ctx, "redis_addr").String()
	password := configs.Value(ctx, "redis_password").String()
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       0,
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	return rdb, nil
}

func initTracer(ctx context.Context) (*sdktrace.TracerProvider, error) {
	host := configs.Value(ctx, "tracer_host").String()
	port := configs.Value(ctx, "tracer_port").String()
	svcName := configs.Value(ctx, "service_name").String()

	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(host+":"+port),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	return sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(svcName),
		)),
	), nil
}
