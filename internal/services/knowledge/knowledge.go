package knowledge

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// Document represents a knowledge document
type Document struct {
	ID       string
	Title    string
	Content  string
	FilePath string
	ModTime  time.Time
	Sections []Section
}

// Section represents a section in a document
type Section struct {
	Title   string
	Content string
	Level   int
}

// Service interface for knowledge base operations
type Service interface {
	LoadKnowledgeBase(ctx context.Context, dir string) error
	SearchDocuments(ctx context.Context, query string, limit int) ([]Document, error)
	GetAllDocuments() []Document
	GetDocument(id string) (*Document, error)
	RefreshKnowledgeBase(ctx context.Context) error
}

// KnowledgeService implements the knowledge base service
type KnowledgeService struct {
	documents   map[string]*Document
	documentsRW sync.RWMutex
	knowledgeDir string
	logger      *logrus.Logger
}

// NewKnowledgeService creates a new knowledge service
func NewKnowledgeService(logger *logrus.Logger) Service {
	return &KnowledgeService{
		documents: make(map[string]*Document),
		logger:    logger,
	}
}

// LoadKnowledgeBase loads all markdown files from the specified directory
func (s *KnowledgeService) LoadKnowledgeBase(ctx context.Context, dir string) error {
	s.knowledgeDir = dir
	s.logger.WithField("dir", dir).Info("Loading knowledge base")
	
	// Create directory if it doesn't exist
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create knowledge directory: %w", err)
	}
	
	s.documentsRW.Lock()
	defer s.documentsRW.Unlock()
	
	// Clear existing documents
	s.documents = make(map[string]*Document)
	
	// Walk through the directory
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		
		// Skip non-markdown files
		if d.IsDir() || !strings.HasSuffix(strings.ToLower(path), ".md") {
			return nil
		}
		
		// Load the document
		doc, err := s.loadDocument(path)
		if err != nil {
			s.logger.WithError(err).WithField("path", path).Warn("Failed to load document")
			return nil // Continue with other files
		}
		
		s.documents[doc.ID] = doc
		s.logger.WithFields(logrus.Fields{
			"id":    doc.ID,
			"title": doc.Title,
			"path":  path,
		}).Debug("Loaded document")
		
		return nil
	})
	
	if err != nil {
		return fmt.Errorf("failed to walk knowledge directory: %w", err)
	}
	
	s.logger.WithField("count", len(s.documents)).Info("Knowledge base loaded")
	return nil
}

// loadDocument loads a single markdown document
func (s *KnowledgeService) loadDocument(path string) (*Document, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}
	
	// Generate document ID from file path
	relPath, _ := filepath.Rel(s.knowledgeDir, path)
	id := strings.TrimSuffix(relPath, filepath.Ext(relPath))
	id = strings.ReplaceAll(id, string(filepath.Separator), "_")
	
	// Parse document
	doc := &Document{
		ID:       id,
		FilePath: path,
		Content:  string(content),
		ModTime:  info.ModTime(),
	}
	
	// Parse title and sections
	s.parseDocument(doc)
	
	return doc, nil
}

// parseDocument parses the markdown content to extract title and sections
func (s *KnowledgeService) parseDocument(doc *Document) {
	lines := strings.Split(doc.Content, "\n")
	doc.Sections = make([]Section, 0)
	
	var currentSection *Section
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		// Check for headers
		if strings.HasPrefix(trimmed, "#") {
			level := 0
			for i, ch := range trimmed {
				if ch == '#' {
					level++
				} else {
					// Extract header text
					headerText := strings.TrimSpace(trimmed[i:])
					
					// First level-1 header is the document title
					if level == 1 && doc.Title == "" {
						doc.Title = headerText
					}
					
					// Create new section
					if currentSection != nil {
						doc.Sections = append(doc.Sections, *currentSection)
					}
					
					currentSection = &Section{
						Title:   headerText,
						Level:   level,
						Content: "",
					}
					break
				}
			}
		} else if currentSection != nil {
			// Add content to current section
			if currentSection.Content != "" {
				currentSection.Content += "\n"
			}
			currentSection.Content += line
		}
	}
	
	// Add the last section
	if currentSection != nil {
		doc.Sections = append(doc.Sections, *currentSection)
	}
	
	// If no title found, use filename
	if doc.Title == "" {
		doc.Title = strings.TrimSuffix(filepath.Base(doc.FilePath), filepath.Ext(doc.FilePath))
		doc.Title = strings.ReplaceAll(doc.Title, "_", " ")
		doc.Title = strings.ReplaceAll(doc.Title, "-", " ")
	}
}

// SearchDocuments searches for documents matching the query
func (s *KnowledgeService) SearchDocuments(ctx context.Context, query string, limit int) ([]Document, error) {
	s.documentsRW.RLock()
	defer s.documentsRW.RUnlock()
	
	query = strings.ToLower(query)
	results := make([]Document, 0)
	
	// Simple keyword matching for now
	// TODO: Implement more sophisticated search (e.g., using embeddings)
	for _, doc := range s.documents {
		score := 0
		
		// Check title match
		if strings.Contains(strings.ToLower(doc.Title), query) {
			score += 10
		}
		
		// Check content match
		contentLower := strings.ToLower(doc.Content)
		score += strings.Count(contentLower, query)
		
		// Check section titles
		for _, section := range doc.Sections {
			if strings.Contains(strings.ToLower(section.Title), query) {
				score += 5
			}
		}
		
		if score > 0 {
			results = append(results, *doc)
			if len(results) >= limit {
				break
			}
		}
	}
	
	return results, nil
}

// GetAllDocuments returns all loaded documents
func (s *KnowledgeService) GetAllDocuments() []Document {
	s.documentsRW.RLock()
	defer s.documentsRW.RUnlock()
	
	docs := make([]Document, 0, len(s.documents))
	for _, doc := range s.documents {
		docs = append(docs, *doc)
	}
	
	return docs
}

// GetDocument returns a specific document by ID
func (s *KnowledgeService) GetDocument(id string) (*Document, error) {
	s.documentsRW.RLock()
	defer s.documentsRW.RUnlock()
	
	doc, exists := s.documents[id]
	if !exists {
		return nil, fmt.Errorf("document not found: %s", id)
	}
	
	return doc, nil
}

// RefreshKnowledgeBase reloads the knowledge base
func (s *KnowledgeService) RefreshKnowledgeBase(ctx context.Context) error {
	return s.LoadKnowledgeBase(ctx, s.knowledgeDir)
}