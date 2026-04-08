package find

import (
	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
)

// FindClientQuery holds parameters for FindClient.
// Field priority: ID > Name > Text. If ID is set, Name and Text are ignored.
// If Name is set, Text is ignored. At least one field must be set.
type FindClientQuery struct {
	ID    int    // Direct lookup by ID (highest priority, ignores Name/Text)
	Name  string // Substring match on client name (ignores Text)
	Text  string // Free-text search across name, code, memo (lowest priority)
	Limit int    // Max results to return (0 = unlimited). Applied at find layer.
	Opts  repository.ReadOptions
}

// FindProjectQuery holds parameters for FindProject.
// Field priority: ID > ClientName > Name > Text. Higher priority fields
// override lower ones. Status is an additional filter applied on top.
type FindProjectQuery struct {
	ID         int    // Direct lookup by ID (highest priority)
	ClientName string // Resolve client name -> client IDs -> filter projects
	Name       string // Project name substring search
	Text       string // Free-text search across name, code, memo (lowest priority)
	Status     string // Additional filter (applied on top of any search mode)
	Limit      int    // Max results to return (0 = unlimited). Applied at find layer.
	Opts       repository.ReadOptions
}

// ClientResult is the aggregated result for a client search.
type ClientResult struct {
	Client   boardapi.ClientEntity         `json:"client"`
	Branches []boardapi.ClientBranchEntity `json:"branches"`
	Contacts []boardapi.ContactEntity      `json:"contacts"`
}

// ProjectResult is the aggregated result for a project search.
type ProjectResult struct {
	Project boardapi.ProjectEntity `json:"project"`
	Client  *boardapi.ClientEntity `json:"client,omitempty"`
}
