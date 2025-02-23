package valfs

import (
	"context"
	"errors"
	"time"

	common "github.com/404wolf/valfs/common"
	"github.com/404wolf/valgo"
)

// ProjectVTFile represents a project object with methods to set attributes
type ProjectVTFile struct {
	authorName  string
	authorId    string
	projectId   string
	name        string
	privacy     string
	description string
	imageUrl    string
	createdAt   time.Time
	url         string
	apiClient   *common.APIClient
}

// ProjectFileOf gets a new ProjectFile instance for a project with an id that already exists
func ProjectFileOf(apiClient *common.APIClient, projectId string) *ProjectVTFile {
	return &ProjectVTFile{apiClient: apiClient, projectId: projectId}
}

// CreateProjectFile creates a new project on the server
func CreateProjectFile(
	ctx context.Context,
	apiClient *common.APIClient,
	name, privacy, description, imageUrl string,
) (*ProjectVTFile, error) {
	return nil, errors.New("not implemented")
}

// DeleteProjectFile deletes a project from the server
func DeleteProjectFile(ctx context.Context, apiClient *common.APIClient, projectId string) error {
	return errors.New("not implemented")
}

// Update updates the project information on the server
func (p *ProjectVTFile) Update(ctx context.Context) error {
	return errors.New("not implemented")
}

// Load retrieves the project details from the server
func (p *ProjectVTFile) Load(ctx context.Context) error {
	project, _, err := p.apiClient.APIClient.BetaAPI.ProjectsGet(ctx, p.projectId).Execute()
	if err != nil {
		return err
	}

	// Load extended properties
	p.setProjectProperties(project)

	return nil
}

// setProjectProperties loads properties from a Project object
func (p *ProjectVTFile) setProjectProperties(project *valgo.Project) {
	p.name = project.GetName()
	p.projectId = project.GetId()
	p.privacy = project.GetPrivacy()
	p.createdAt = project.GetCreatedAt()
	p.url = project.GetValTownUrl()
	if project.Description.IsSet() {
		p.description = project.GetDescription()
	}
	if project.ImageUrl.IsSet() {
		p.imageUrl = project.GetImageUrl()
	}

	// Set author information
	authorData := project.GetAuthor()
	p.authorId = authorData.GetId()
	p.authorName = authorData.GetUsername()
}

// GetId returns the project ID
func (p *ProjectVTFile) GetId() string {
	return p.projectId
}

// GetName returns the project name
func (p *ProjectVTFile) GetName() string {
	return p.name
}

// SetName sets the name of the project
func (p *ProjectVTFile) SetName(name string) {
	p.name = name
}

// GetPrivacy returns the project privacy setting
func (p *ProjectVTFile) GetPrivacy() string {
	return p.privacy
}

// SetPrivacy sets the privacy of the project
func (p *ProjectVTFile) SetPrivacy(privacy string) {
	p.privacy = privacy
}

// GetDescription returns the project description
func (p *ProjectVTFile) GetDescription() string {
	return p.description
}

// SetDescription sets the project description
func (p *ProjectVTFile) SetDescription(description string) {
	p.description = description
}

// GetImageUrl returns the project image URL
func (p *ProjectVTFile) GetImageUrl() string {
	return p.imageUrl
}

// SetImageUrl sets the project image URL
func (p *ProjectVTFile) SetImageUrl(imageUrl string) {
	p.imageUrl = imageUrl
}

// GetCreatedAt returns the creation time of the project
func (p *ProjectVTFile) GetCreatedAt() time.Time {
	return p.createdAt
}

// GetUrl returns the project's URL
func (p *ProjectVTFile) GetUrl() string {
	return p.url
}

// GetAuthorName returns the project's author's name
func (p *ProjectVTFile) GetAuthorName() string {
	return p.authorName
}

// GetAuthorId returns the project's author's Id
func (p *ProjectVTFile) GetAuthorId() string {
	return p.authorId
}
