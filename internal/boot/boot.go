package boot

import (
	"context"
	"fmt"
	"log"
	"net"

	"github.com/helthtech/core-health/internal/migration"
	"github.com/helthtech/core-health/internal/repository"
	"github.com/helthtech/core-health/internal/seed"
	"github.com/helthtech/core-health/internal/server"
	"github.com/helthtech/core-health/internal/service"
	pb "github.com/helthtech/core-health/pkg/proto/health"
	"github.com/nats-io/nats.go"
	"github.com/porebric/configs"
	"github.com/porebric/resty"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"google.golang.org/grpc"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Run(ctx context.Context) error {
	db, err := initDB(ctx)
	if err != nil {
		return fmt.Errorf("init db: %w", err)
	}
	if err = migration.Run(db); err != nil {
		return fmt.Errorf("migration: %w", err)
	}
	if err = seed.Run(db); err != nil {
		log.Printf("seed (non-fatal): %v", err)
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
		log.Printf("tracer init failed (non-fatal): %v", err)
	} else {
		otel.SetTracerProvider(tp)
		defer func() { _ = tp.Shutdown(context.Background()) }()
	}

	repo := repository.NewHealthRepository(db)
	svc := service.NewHealthService(repo, nc, rdb)

	// Start in-memory cache refresh loop.
	svc.StartCache(ctx)

	// Start schedulers in background.
	go svc.RunDailyScheduler(ctx, []string{"telegram", "max"})
	go svc.RunExpiryScheduler(ctx, []string{"telegram", "max"})
	go svc.RunAlarmScheduler(ctx, []string{"telegram", "max"})
	go svc.RunWeeklyScheduler(ctx)

	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)
	pb.RegisterHealthServiceServer(grpcServer, server.NewHealthServer(svc))

	grpcPort := configs.Value(ctx, "grpc_port").String()
	lis, err := net.Listen("tcp", "0.0.0.0:"+grpcPort)
	if err != nil {
		return fmt.Errorf("listen grpc: %w", err)
	}

	go func() {
		log.Printf("gRPC server listening on :%s", grpcPort)
		if err := grpcServer.Serve(lis); err != nil {
			log.Printf("gRPC serve error: %v", err)
		}
	}()

	router := resty.NewRouter(nil, nil)
	resty.RunServer(ctx, router, func(ctx context.Context) error {
		grpcServer.GracefulStop()
		nc.Close()
		return nil
	})

	return nil
}

func initDB(ctx context.Context) (*gorm.DB, error) {
	dsn := configs.Value(ctx, "db_dsn").String()
	return gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
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
