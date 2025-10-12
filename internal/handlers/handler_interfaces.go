package handlers

// ProjectCacheClearer defines the interface for clearing Jira project cache.
type ProjectCacheClearer interface {
	ClearProjectsCache() error
}

// ProjectIssueGetter defines the interface for fetching issues for a specific project.
type ProjectIssueGetter interface {
	GetProjectIssues(projectKey string) error
}

// SpacePageGetter defines the interface for fetching pages for a specific Confluence space.
type SpacePageGetter interface {
	GetSpacePages(spaceKey string) error
}

// SpaceCacheClearer defines the interface for clearing Confluence space cache.
type SpaceCacheClearer interface {
	ClearSpacesCache() error
}

// DataClearer defines the interface for clearing all data from a service.
type DataClearer interface {
	ClearAllData() error
}
