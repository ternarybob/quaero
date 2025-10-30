package interfaces

// JobListOptions for listing jobs
type JobListOptions struct {
	SourceType string
	Limit      int
	Offset     int
	OrderBy    string // created_at, updated_at, title
	OrderDir   string // asc, desc
	Status     string // Filter by job status (for job listings)
	EntityType string // Filter by entity type (for job listings)
	ParentID   string // Filter by parent job ID (empty = no filter, "root" = only root jobs, specific ID = children of that parent)
	Grouped    bool   // Whether to group jobs by parent-child relationship (default: false for flat list)
}
