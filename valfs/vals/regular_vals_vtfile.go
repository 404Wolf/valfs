package valfs

import (
	"context"
	"time"

	common "github.com/404wolf/valfs/common"
	"github.com/404wolf/valgo"
	"github.com/hanwen/go-fuse/v2/fs"
)

// RegularValVTFile represents a Val Town val object that can be manipulated as a file.
// It implements both the VTFile and ValVTFile interfaces, providing methods for
// reading, writing, and managing val attributes.
type RegularValVTFile struct {
	// Basic file system attributes
	container *VTFileContainer
	path      string

	// Val-specific attributes - required fields
	valId          string
	name           string
	valType        string
	code           string
	privacy        string
	version        int32
	moduleLink     string
	apiClient      *common.APIClient
	createdAt      time.Time
	url            string
	likeCount      int32
	referenceCount int32
	public         bool

	// Optional fields
	authorName   *string
	authorId     *string
	readme       *string
	endpointLink *string
	versionsLink *string
	inode        *fs.Inode
}

// RegularValVTFileOf creates a new ValVTFile instance for an existing val ID.
// It returns a minimal instance that needs to be loaded before use.
func RegularValVTFileOf(apiClient *common.APIClient, valId string) ValVTFile {
	return &RegularValVTFile{apiClient: apiClient, valId: valId}
}

// CreateNewValVTFile creates a new val on the Val Town server with the specified parameters.
// It returns a fully initialized RegularValVTFile instance or an error if creation fails.
func CreateNewValVTFile(
	ctx context.Context,
	apiClient *common.APIClient,
	valType VTFileType,
	code, name, privacy string,
) (*RegularValVTFile, error) {
	createReq := valgo.NewValsCreateRequest(code)
	createReq.SetName(name)
	createReq.SetType(string(valType))
	createReq.SetPrivacy(privacy)

	extVal, _, err := apiClient.APIClient.ValsAPI.ValsCreate(ctx).ValsCreateRequest(*createReq).Execute()
	if err != nil {
		return nil, err
	}

	val := RegularValVTFileOf(apiClient, extVal.GetId()).(*RegularValVTFile)
	val.setExtendedValProperties(extVal)
	return val, nil
}

// DeleteValVTFile removes a val from the Val Town server.
// It returns an error if the deletion fails.
func DeleteValVTFile(ctx context.Context, apiClient *common.APIClient, valId string) error {
	_, err := apiClient.APIClient.ValsAPI.ValsDelete(ctx, valId).Execute()
	return err
}

// GetContainer returns the VTFileContainer that this val belongs to.
func (v *RegularValVTFile) GetContainer() *VTFileContainer {
	return v.container
}

// GetPath returns the filesystem path where this val is mounted.
func (v *RegularValVTFile) GetPath() string {
	return v.path
}

// GetPackedText returns the val's contents as a byte slice, including frontmatter and code.
// Returns an empty byte slice if there's an error getting the text representation.
func (v *RegularValVTFile) GetPackedText() []byte {
	text, err := v.GetAsPackedText()
	if err != nil {
		common.Logger.Errorf("failed to get val as text: %v", err)
		return []byte{}
	}
	if text == nil {
		return []byte{}
	}
	return []byte(*text)
}

// SetPath updates the filesystem path where this val is mounted.
func (v *RegularValVTFile) SetPath(path string) {
	v.path = path
}

// SetType updates the val's type (e.g., "js", "python", etc.).
func (v *RegularValVTFile) SetType(type_ string) {
	v.valType = type_
}

// GetApiUrl returns the API URL for this val.
func (v *RegularValVTFile) GetApiUrl() string {
	return v.url
}

// GetModuleUrl returns the module URL for this val.
func (v *RegularValVTFile) GetModuleUrl() string {
	return v.moduleLink
}

// GetDeployedUrl returns the endpoint URL for this val if it exists.
func (v *RegularValVTFile) GetDeployedUrl() *string {
	return v.endpointLink
}

// UpdateFromText parses the provided text representation of a val and updates
// its properties accordingly. The text should include frontmatter and code.
func (v *RegularValVTFile) UpdateFromPackedText(ctx context.Context, text string) error {
	tempPackage := NewValPackage(v, false, false)

	if err := tempPackage.UpdateVal(text); err != nil {
		common.Logger.Errorf("failed to update val from text: %v", err)
		return err
	}

	if err := v.Save(ctx); err != nil {
		common.Logger.Errorf("failed to save val changes: %v", err)
		return err
	}

	return nil
}

// GetAsText returns a complete text representation of the val,
// including frontmatter and code.
func (v *RegularValVTFile) GetAsPackedText() (*string, error) {
	valPackage := NewValPackage(v, false, false)
	text, err := valPackage.ToText()
	if err != nil {
		common.Logger.Errorf("failed to get val as text: %v", err)
		return nil, err
	}
	return text, nil
}

// Save updates the val's information on the Val Town server.
func (v *RegularValVTFile) Save(ctx context.Context) error {
	common.Logger.Info("Saving val", "valId", v.valId)

	// Update metadata
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
	if v.readme != nil {
		updateReq.SetReadme(*v.readme)
	}

	common.Logger.Debug("Updating val metadata", "valId", v.valId)
	if _, err := v.apiClient.APIClient.ValsAPI.ValsUpdate(ctx, v.valId).
		ValsUpdateRequest(*updateReq).Execute(); err != nil {
		common.Logger.Error("Failed to update val metadata", "valId", v.valId, "error", err)
		return err
	}

	// Update code separately (API limitation)
	code := v.code
	if code == "" {
		code = " " // Val Town requires at least one character
	}
	valCreateReqData := valgo.NewValsCreateRequest(code)

	common.Logger.Debug("Creating new val version", "valId", v.valId)
	extVal, _, err := v.apiClient.APIClient.ValsAPI.ValsCreateVersion(ctx, v.GetId()).
		ValsCreateRequest(*valCreateReqData).
		Execute()
	if err != nil {
		common.Logger.Error("Failed to create new version", "valId", v.valId, "error", err)
		return err
	}

	v.setExtendedValProperties(extVal)
	common.Logger.Info("Successfully updated val", "valId", v.valId)
	return nil
}

// Load retrieves the val's details from the Val Town server.
func (v *RegularValVTFile) Load(ctx context.Context) error {
	common.Logger.Info("Loading val details", "valId", v.valId)
	val, _, err := v.apiClient.APIClient.ValsAPI.ValsGet(ctx, v.valId).Execute()
	if err != nil {
		common.Logger.Error("Failed to load val details", "valId", v.valId, "error", err)
		return err
	}

	v.setExtendedValProperties(val)
	common.Logger.Info("Successfully loaded val details", "valId", v.valId)
	return nil
}

// setExtendedValProperties updates the val's properties from an ExtendedVal object.
func (v *RegularValVTFile) setExtendedValProperties(val *valgo.ExtendedVal) {
	// Required fields
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
	v.moduleLink = val.Links.Module

	// Optional fields
	if val.Code.IsSet() {
		v.code = val.GetCode()
	}

	if val.Readme.IsSet() {
		readme := val.GetReadme()
		v.readme = &readme
	} else {
		v.readme = nil
	}

	if val.Links.Endpoint != nil {
		v.endpointLink = val.Links.Endpoint
	} else {
		v.endpointLink = nil
	}

	if val.Links.Versions != "" {
		versionsLink := val.Links.Versions
		v.versionsLink = &versionsLink
	} else {
		v.versionsLink = nil
	}

	if author := val.Author; author.IsSet() {
		authorData := author.Get()
		id := authorData.GetId()
		username := authorData.GetUsername()
		v.authorId = &id
		v.authorName = &username
	} else {
		v.authorId = nil
		v.authorName = nil
	}
}

// ListValVTFiles retrieves all vals for the authenticated user with pagination.
func ListValVTFiles(ctx context.Context, apiClient *common.APIClient) ([]*RegularValVTFile, error) {
	common.Logger.Info("Fetching user information")
	meResp, _, err := apiClient.APIClient.MeAPI.MeGet(ctx).Execute()
	if err != nil {
		common.Logger.Error("Failed to get user information", "error", err)
		return nil, err
	}

	var allBasicVals []valgo.BasicVal
	currentOffset := int32(0)

	common.Logger.Info("Starting to fetch vals", "userId", meResp.GetId())
	for {
		common.Logger.Debug("Fetching page of vals", "offset", currentOffset, "limit", ApiPageLimit)
		basicVals, _, err := apiClient.APIClient.UsersAPI.UsersVals(ctx, meResp.GetId()).
			Offset(currentOffset).
			Limit(ApiPageLimit).
			Execute()

		if err != nil {
			common.Logger.Error("Failed to fetch vals page", "offset", currentOffset, "error", err)
			return nil, err
		}

		if len(basicVals.Data) == 0 {
			break
		}

		allBasicVals = append(allBasicVals, basicVals.Data...)
		common.Logger.Debug("Fetched vals page", "count", len(basicVals.Data))

		if int32(len(basicVals.Data)) < ApiPageLimit {
			break
		}

		currentOffset += ApiPageLimit
	}

	vals := make([]*RegularValVTFile, 0, len(allBasicVals))
	for _, val := range allBasicVals {
		regularValVTFile := RegularValVTFileOf(apiClient, val.GetId()).(*RegularValVTFile)
		regularValVTFile.SetName(val.Name)
		regularValVTFile.SetValType(val.Type)
		regularValVTFile.SetCode(val.GetCode())
		regularValVTFile.SetPrivacy(val.Privacy)
		regularValVTFile.SetReadme(nil)
		vals = append(vals, regularValVTFile)
	}
	common.Logger.Info("Successfully fetched all vals", "count", len(vals))
	return vals, nil
}

// Interface implementation methods

func (v *RegularValVTFile) GetInode() *fs.Inode {
	return v.inode
}

func (v *RegularValVTFile) GetVersionsLink() *string {
	return v.versionsLink
}

func (v *RegularValVTFile) GetModuleLink() string {
	return v.moduleLink
}

func (v *RegularValVTFile) GetEndpointLink() *string {
	return v.endpointLink
}

func (v *RegularValVTFile) GetDeployedLink() *string {
	return v.endpointLink
}

func (v *RegularValVTFile) GetId() string {
	return v.valId
}

func (v *RegularValVTFile) GetName() string {
	return v.name
}

func (v *RegularValVTFile) GetType() VTFileType {
	return VTFileType(v.valType)
}

func (v *RegularValVTFile) GetCode() string {
	return v.code
}

func (v *RegularValVTFile) GetPrivacy() string {
	return v.privacy
}

func (v *RegularValVTFile) GetReadme() *string {
	return v.readme
}

func (v *RegularValVTFile) GetVersion() int32 {
	return v.version
}

func (v *RegularValVTFile) GetAuthorName() *string {
	return v.authorName
}

func (v *RegularValVTFile) GetAuthorId() *string {
	return v.authorId
}

func (v *RegularValVTFile) GetCreatedAt() time.Time {
	return v.createdAt
}

func (v *RegularValVTFile) GetUrl() string {
	return v.url
}

func (v *RegularValVTFile) GetLikeCount() int32 {
	return v.likeCount
}

func (v *RegularValVTFile) GetReferenceCount() int32 {
	return v.referenceCount
}

func (v *RegularValVTFile) SetName(name string) {
	v.name = name
}

func (v *RegularValVTFile) SetValType(valType string) {
	v.valType = valType
}

func (v *RegularValVTFile) SetCode(code string) {
	v.code = code
}

func (v *RegularValVTFile) SetPrivacy(privacy string) {
	v.privacy = privacy
}

func (v *RegularValVTFile) SetReadme(readme *string) {
	v.readme = readme
}
