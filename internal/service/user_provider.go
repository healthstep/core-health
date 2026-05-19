package service

import (
	"context"

	"github.com/google/uuid"
	userspb "github.com/helthtech/core-users/pkg/proto/users"
)

type UserContext struct {
	Sex       string
	BirthDate string
	Advanced  bool
}

type UserContextProvider interface {
	GetUserContext(ctx context.Context, userID uuid.UUID) (UserContext, error)
}

type GRPCUserContextProvider struct {
	client userspb.UserServiceClient
}

func NewGRPCUserContextProvider(client userspb.UserServiceClient) *GRPCUserContextProvider {
	return &GRPCUserContextProvider{client: client}
}

func (p *GRPCUserContextProvider) GetUserContext(ctx context.Context, userID uuid.UUID) (UserContext, error) {
	resp, err := p.client.GetUser(ctx, &userspb.GetUserRequest{UserId: userID.String()})
	if err != nil {
		return UserContext{}, err
	}
	return UserContext{
		Sex:       resp.GetSex(),
		BirthDate: resp.GetBirthDate(),
		Advanced:  resp.GetAdvanced(),
	}, nil
}
