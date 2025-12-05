package model

// Folder represents a container for bookmarks and other folders.
type Folder struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	ParentID *string `json:"parentId"` // nil = root level
	Pinned   bool    `json:"pinned"`
}

// NewFolderParams holds parameters for creating a new Folder.
type NewFolderParams struct {
	Name     string
	ParentID *string
}

// NewFolder creates a Folder with generated UUID.
func NewFolder(params NewFolderParams) Folder {
	return Folder{
		ID:       GenerateUUID(),
		Name:     params.Name,
		ParentID: params.ParentID,
	}
}
