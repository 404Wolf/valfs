package valfs

import (
	"context"
	"time"

	common "github.com/404wolf/valfs/common"
	"github.com/404wolf/valgo"
)

// BaseVal represents a val object with methods to set attributes
type ValDirVal struct {
	authorName     string
	authorId       string
	valId          string
	name           string
	valType        string
	code           string
	privacy        string
	readme         string
	version        int32
	endpointLink   string
	moduleLink     string
	versionsLink   string
	apiClient      *common.APIClient
	createdAt      time.Time
	public         bool
	url            string
	likeCount      int32
	referenceCount int32
}

// ValDirValOf gets a new Val instance for a val with an id that already
// exists
func ValDirValOf(apiClient *common.APIClient, valId string) Val {
	return &ValDirVal{apiClient: apiClient, valId: valId}
}

// CreateValDirVal creates a new val on the server
func CreateValDirVal(
	ctx context.Context,
	apiClient *common.APIClient,
	valType ValType,
	code, name, privacy string,
) (Val, error) {
	// Create the val
	createReq := valgo.NewValsCreateRequest(code)
	createReq.SetName(name)
	createReq.SetType(string(valType))
	createReq.SetPrivacy(privacy)

	extVal, _, err := apiClient.APIClient.ValsAPI.ValsCreate(ctx).ValsCreateRequest(*createReq).Execute()
	if err != nil {
		return nil, err
	}

	return ValDirValOf(apiClient, extVal.GetId()), nil
}

// DeleteValDirVil deletes a val from the server
func DeleteValDirVal(ctx context.Context, apiClient *common.APIClient, valId string) error {
	_, err := apiClient.APIClient.ValsAPI.ValsDelete(ctx, valId).Execute()
	return err
}

// Update updates the val information on the server
func (v *ValDirVal) Update(ctx context.Context) error {
	// If the metadata changed, update the metadata
	updateReq := valgo.NewValsUpdateRequest()

	if v.name != "" {
		updateReq.SetName(v.name)
	}

	if v.valType != "" {
		updateReq.SetType(v.valType)
	}

	if v.privacy != "" {
		updateReq.SetPrivacy(v.privacy)
	}

	if v.readme != "" {
		updateReq.SetReadme(v.readme)
	}

	_, err := v.apiClient.APIClient.ValsAPI.ValsUpdate(ctx, v.valId).ValsUpdateRequest(*updateReq).Execute()
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
	extVal, _, err := v.apiClient.APIClient.ValsAPI.ValsCreateVersion(ctx, v.GetId()).
		ValsCreateRequest(*valCreateReqData).
		Execute()
	if err != nil {
		common.Logger.Error("Error creating new version", "error", err)
		return err
	}
	v.loadExtendedValProperties(extVal)

	common.Logger.Info("Successfully updated val code", "valId", v.GetId())
	return nil
}

// Load retrieves the val details from the server
func (v *ValDirVal) Load(ctx context.Context) error {
	val, _, err := v.apiClient.APIClient.ValsAPI.ValsGet(ctx, v.valId).Execute()
	if err != nil {
		return err
	}

	// Load extended properties
	v.loadExtendedValProperties(val)

	return nil
}

// loadExtendedValProperties loads extended properties from a Val object
func (v *ValDirVal) loadExtendedValProperties(val *valgo.ExtendedVal) {
	// Set basic fields
	v.name = val.Name
	v.valId = val.Id
	v.valType = val.Type
	v.version = val.Version
	v.privacy = val.Privacy
	v.public = val.Public
	v.createdAt = val.CreatedAt
	v.url = val.Url
	v.likeCount = val.LikeCount
	v.referenceCount = val.ReferenceCount

	// Set code, ensuring it's handled properly as NullableString
	if val.Code.IsSet() {
		v.code = val.GetCode()
	}

	// Set readme, ensuring it's handled properly as NullableString
	if val.Readme.IsSet() {
		v.readme = val.GetReadme()
	}

	// Set links
	links := val.Links
	if links.Endpoint != nil {
		v.endpointLink = *links.Endpoint
	}
	v.moduleLink = links.Module
	v.versionsLink = links.Versions

	// Set author information
	if author := val.Author; author.IsSet() {
		authorData := author.Get()
		v.authorId = authorData.GetId()
		v.authorName = authorData.GetUsername()
	}
}

// ListValDirVals is a standalone function to list vals with pagination
func ListValDirVals(ctx context.Context, apiClient *common.APIClient) ([]Val, error) {
	meResp, _, err := apiClient.APIClient.MeAPI.MeGet(ctx).Execute()
	if err != nil {
		return nil, err
	}

	var allBasicVals []valgo.BasicVal
	currentOffset := int32(0)

	for {
		basicVals, _, err := apiClient.APIClient.UsersAPI.UsersVals(ctx, meResp.GetId()).
			Offset(currentOffset).
			Limit(ApiPageLimit).
			Execute()

		if err != nil {
			return nil, err
		}

		// If no more data, break the loop
		if len(basicVals.Data) == 0 {
			break
		}

		// Append this page's data to our collection
		allBasicVals = append(allBasicVals, basicVals.Data...)

		// If we got less than the limit, we've hit the end
		if int32(len(basicVals.Data)) < ApiPageLimit {
			break
		}

		// Move offset for next iteration
		currentOffset += ApiPageLimit
	}

	// Convert each of the basic vals into Val instances
	vals := make([]Val, 0, len(allBasicVals))
	for _, val := range allBasicVals {
		valDirVal := ValDirValOf(apiClient, val.GetId())
		valDirVal.SetName(val.Name)
		valDirVal.SetValType(val.Type)
		valDirVal.SetCode(val.GetCode())
		valDirVal.SetPrivacy(val.Privacy)
		valDirVal.SetReadme("") // No readme in BasicVal, setting empty
		vals = append(vals, valDirVal)
	}
	return vals, nil
}

// GetVersionsLink returns the link to the val's versions
func (v *ValDirVal) GetVersionsLink() string {
	return v.versionsLink
}

// GetModuleLink returns the link to the val's module
func (v *ValDirVal) GetModuleLink() string {
	return v.moduleLink
}

// GetEndpointLink returns the link to the val's endpoint
func (v *ValDirVal) GetEndpointLink() string {
	return v.endpointLink
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

// GetAuthorName returns the val's author's name
func (v *ValDirVal) GetAuthorName() string {
	return v.authorName
}

// GetAuthorId returns the val's author's Id
func (v *ValDirVal) GetAuthorId() string {
	return v.authorId
}

// GetCreatedAt returns the creation time of the val
func (v *ValDirVal) GetCreatedAt() time.Time {
	return v.createdAt
}

// GetUrl returns the val's URL
func (v *ValDirVal) GetUrl() string {
	return v.url
}

// GetLikeCount returns the number of likes the val has
func (v *ValDirVal) GetLikeCount() int32 {
	return v.likeCount
}

// GetReferenceCount returns the number of references to the val
func (v *ValDirVal) GetReferenceCount() int32 {
	return v.referenceCount
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
