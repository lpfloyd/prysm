package accounts

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared/mock"
)

func TestFetchAccountStatuses_OK(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	pubkeys := make([][]byte, 10000)
	for i := 0; i < 10000; i++ {
		pubkeys[i] = []byte{byte(i)}
	}

	mockClient := mock.NewMockBeaconNodeValidatorClient(ctrl)
	mockClient.EXPECT().MultipleValidatorStatus(
		gomock.Any(),
		&ethpb.MultipleValidatorStatusRequest{PublicKeys: pubkeys},
	)
	_, err := FetchAccountStatuses(ctx, mockClient, pubkeys)
	if err != nil {
		t.Fatalf("FetchAccountStatuses failed with error: %v.", err)
	}
}
