package valfs

import (
	"context"

	common "github.com/404wolf/valfs/common"
	"github.com/404wolf/valgo"
)

// BaseVal represents a val object with methods to set attributes
type ValDirVal struct {
	valId   string
	name    string
	valType string
	code    string
	privacy string
	readme  string
	version int32
	client  *common.Client
}

// NewVal initializes a new Val instance
func NewValDirVal(client *common.Client, valId string) Val {
	return &ValDirVal{
		client: client,
		valId:  valId,
	}
}

// Update updates the val information on the server
func (v *ValDirVal) Update(ctx context.Context) error {
	// If the metadata changed, update the metadata
	updateReq := valgo.NewValsUpdateRequest()
	updateReq.SetName(v.name)
	updateReq.SetType(v.valType)
	updateReq.SetPrivacy(v.privacy)
	updateReq.SetReadme(v.readme)
	_, err := v.client.APIClient.ValsAPI.ValsUpdate(ctx, v.valId).ValsUpdateRequest(*updateReq).Execute()
	if err != nil {
		return err
	}

	// If the code changed, update the code seperately, because of a bug in the
	// val town API where you cannot set the code and metadata in the same request
	valCreateReqData := valgo.NewValsCreateRequest(v.code)
	// Val town requires at least one character
	if len(valCreateReqData.Code) == 0 {
		valCreateReqData.Code = " "
	}

	// Create new version
	_, _, err = v.client.APIClient.ValsAPI.ValsCreateVersion(ctx, v.valId).
		ValsCreateRequest(*valCreateReqData).
		Execute()
	if err != nil {
		common.Logger.Error("Error creating new version", "error", err)
		return err
	}

	common.Logger.Info("Successfully updated val code", "valId", v.valId)
	return nil
}

// Get retrieves the val details from the server
func (v *ValDirVal) Get(ctx context.Context) error {
	val, _, err := v.client.APIClient.ValsAPI.ValsGet(ctx, v.valId).Execute()
	if err != nil {
		return err
	}

	v.name = val.GetName()
	v.valType = val.GetType()
	v.privacy = val.GetPrivacy()
	v.readme = val.GetReadme()
	v.version = val.GetVersion()

	return nil
}

// List retrieves all vals for the authenticated user
func (v *ValDirVal) List(ctx context.Context) error {
	meResp, _, err := v.client.APIClient.MeAPI.MeGet(ctx).Execute()
	if err != nil {
		return err
	}

	vals, _, err := v.client.APIClient.UsersAPI.UsersVals(ctx, meResp.GetId()).
		Execute()
	if err != nil {
		return err
	}

	// Since the interface doesn't specify a return value for List,
	// we'll just log success and return nil
	common.Logger.Info("Successfully retrieved vals", "count", len(vals.Data))
	return nil
}

// ListVals is a standalone function to list vals with pagination
func ListVals(ctx context.Context, client *common.Client, offset, limit int32) ([]valgo.BasicVal, error) {
	meResp, _, err := client.APIClient.MeAPI.MeGet(ctx).Execute()
	if err != nil {
		return nil, err
	}

	vals, _, err := client.APIClient.UsersAPI.UsersVals(ctx, meResp.GetId()).
		Offset(offset).
		Limit(limit).
		Execute()
	if err != nil {
		return nil, err
	}

	return vals.Data, nil
}

// GetID returns the val ID
func (v *ValDirVal) GetId() string {
	return v.valId
}

// GetName returns the val name
func (v *ValDirVal) GetName() string {
	return v.name
}

// GetValType returns the val type
func (v *ValDirVal) GetValType() ValType {
	return ValType(v.valType)
}

// GetCode returns the val code
func (v *ValDirVal) GetCode() string {
	return v.code
}

// GetPrivacy returns the val privacy setting
func (v *ValDirVal) GetPrivacy() string {
	return v.privacy
}

// GetReadme returns the val readme
func (v *ValDirVal) GetReadme() string {
	return v.readme
}

// GetVersion returns the val version
func (v *ValDirVal) GetVersion() int32 {
	return v.version
}

// SetName sets the name of the val
func (v *ValDirVal) SetName(name string) {
	v.name = name
}

// SetValType sets the type of the val
func (v *ValDirVal) SetValType(valType string) {
	v.valType = valType
}

// SetCode sets the code of the val
func (v *ValDirVal) SetCode(code string) {
	v.code = code
}

// SetPrivacy sets the privacy of the val
func (v *ValDirVal) SetPrivacy(privacy string) {
	v.privacy = privacy
}

// SetReadme sets the readme of the val
func (v *ValDirVal) SetReadme(readme string) {
	v.readme = readme
}
