package search

import "context"

type LineMatch struct {
	LineNumber int    `json:"line_number"`
	LineText   string `json:"line_text"`
	ColStart   int    `json:"col_start"`
	ColEnd     int    `json:"col_end"`
}

type FileMatches struct {
	RelPath string      `json:"rel_path"`
	AbsPath string      `json:"abs_path"`
	Matches []LineMatch `json:"matches"`
}

type RepoMatches struct {
	RepoName    string        `json:"repo_name"`
	RepoPath    string        `json:"repo_path"`
	FileMatches []FileMatches `json:"file_matches"`
	TotalHits   int           `json:"total_hits"`
}

type Engine interface {
	Search(ctx context.Context, query string, paths []string) ([]RepoMatches, error)
	Available() bool
}

type NodeKind int

const (
	NodeRepo NodeKind = iota
	NodeFile
	NodeMatch
)

type SearchNode struct {
	Kind     NodeKind
	Title    string
	Depth    int
	Expanded bool
	RepoName string
	RepoPath string
	FilePath string
	AbsPath  string
	Match    *LineMatch
	Children []*SearchNode
}

// BuildTree converts RepoMatches into hierarchical tree nodes
func BuildTree(repos []RepoMatches) []*SearchNode {
	var roots []*SearchNode
	for _, r := range repos {
		repoNode := &SearchNode{
			Kind:     NodeRepo,
			Title:    r.RepoName,
			Depth:    0,
			Expanded: true,
			RepoName: r.RepoName,
			RepoPath: r.RepoPath,
		}

		for _, f := range r.FileMatches {
			fileNode := &SearchNode{
				Kind:     NodeFile,
				Title:    f.RelPath,
				Depth:    1,
				Expanded: true,
				RepoName: r.RepoName,
				RepoPath: r.RepoPath,
				FilePath: f.RelPath,
				AbsPath:  f.AbsPath,
			}

			for _, m := range f.Matches {
				matchCopy := m
				matchNode := &SearchNode{
					Kind:     NodeMatch,
					Title:    m.LineText,
					Depth:    2,
					RepoName: r.RepoName,
					RepoPath: r.RepoPath,
					FilePath: f.RelPath,
					AbsPath:  f.AbsPath,
					Match:    &matchCopy,
				}
				fileNode.Children = append(fileNode.Children, matchNode)
			}
			repoNode.Children = append(repoNode.Children, fileNode)
		}
		roots = append(roots, repoNode)
	}
	return roots
}

// FlattenVisibleNodes returns a flat slice of only the expanded/visible nodes
func FlattenVisibleNodes(roots []*SearchNode) []*SearchNode {
	var visible []*SearchNode
	for _, root := range roots {
		visible = append(visible, root)
		if root.Expanded {
			for _, file := range root.Children {
				visible = append(visible, file)
				if file.Expanded {
					visible = append(visible, file.Children...)
				}
			}
		}
	}
	return visible
}
