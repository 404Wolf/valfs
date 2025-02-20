package valfs

import (
	"context"

	common "github.com/404wolf/valfs/common"
	"github.com/404wolf/valgo"
)

// ValDirOperations implements ValOperations interface
type ValDirOperations struct {
	client *common.Client
}

// NewValOperations creates a new ValOperations instance
func NewValDirOperations(client *common.Client) ValOperations {
	return &ValDirOperations{
		client: client,
	}
}

// Implementation of ValOperations interface
func (v *ValDirOperations) Create(
	ctx context.Context,
	name, valType, code, privacy string,
) (*valgo.ExtendedVal, error) {
	createReq := valgo.NewValsCreateRequest(code)
	createReq.SetName(name)
	createReq.SetType(valType)
	createReq.SetPrivacy(privacy)

	val, _, err := v.client.APIClient.ValsAPI.ValsCreate(ctx).ValsCreateRequest(*createReq).Execute()
	if err != nil {
		return nil, err
	}
	return val, nil
}

func (v *ValDirOperations) Read(ctx context.Context, valId string) (*valgo.ExtendedVal, error) {
	val, _, err := v.client.APIClient.ValsAPI.ValsGet(ctx, valId).Execute()
	if err != nil {
		return nil, err
	}
	return val, nil
}

func (v *ValDirOperations) Update(
	ctx context.Context,
	valId string,
	extVal *valgo.ExtendedVal,
) (*valgo.ExtendedVal, error) {
	updateReq := valgo.NewValsUpdateRequest()
	updateReq.SetName(extVal.Name)
	updateReq.SetType(extVal.Type)
	updateReq.SetPrivacy(extVal.Privacy)

	_, err := v.client.APIClient.ValsAPI.ValsUpdate(ctx, valId).ValsUpdateRequest(*updateReq).Execute()
	if err != nil {
		return nil, err
	}

	extVal, _, err = v.client.APIClient.ValsAPI.ValsGet(ctx, valId).Execute()

	return extVal, nil
}

func (v *ValDirOperations) UpdateCode(ctx context.Context, valId string, code string) error {
	// Create version request with the new code
	valCreateReqData := valgo.NewValsCreateRequest(code)
	// Val town requires at least one character
	if len(valCreateReqData.Code) == 0 {
		valCreateReqData.Code = " "
	}

	// Create new version
	_, _, err := v.client.APIClient.ValsAPI.ValsCreateVersion(ctx, valId).
		ValsCreateRequest(*valCreateReqData).
		Execute()
	if err != nil {
		common.Logger.Error("Error creating new version", "error", err)
		return err
	}

	common.Logger.Info("Successfully updated val code", "valId", valId)
	return nil
}

func (v *ValDirOperations) Delete(ctx context.Context, valId string) error {
	_, err := v.client.APIClient.ValsAPI.ValsDelete(ctx, valId).Execute()
	return err
}

func (v *ValDirOperations) List(
	ctx context.Context,
	offset, limit int32,
) ([]valgo.BasicVal, error) {
	meResp, _, err := v.client.APIClient.MeAPI.MeGet(ctx).Execute()
	if err != nil {
		return nil, err
	}

	vals, _, err := v.client.APIClient.UsersAPI.UsersVals(ctx, meResp.GetId()).
		Offset(offset).
		Limit(limit).
		Execute()
	if err != nil {
		return nil, err
	}

	return vals.Data, nil
}
