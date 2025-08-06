package knowledge

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/sirupsen/logrus"
)

// EmbeddingService provides text embedding functionality
type EmbeddingService interface {
	GetEmbedding(text string) ([]float32, error)
	CosineSimilarity(a, b []float32) float32
}

// SimpleEmbeddingService implements a simple TF-IDF based embedding
// For production, consider using OpenAI embeddings or other vector models
type SimpleEmbeddingService struct {
	vocabulary map[string]int
	idf        map[string]float64
}

// NewSimpleEmbeddingService creates a new simple embedding service
func NewSimpleEmbeddingService() *SimpleEmbeddingService {
	return &SimpleEmbeddingService{
		vocabulary: make(map[string]int),
		idf:        make(map[string]float64),
	}
}

// BuildVocabulary builds vocabulary from documents
func (s *SimpleEmbeddingService) BuildVocabulary(documents []Document) {
	// Reset vocabulary
	s.vocabulary = make(map[string]int)
	s.idf = make(map[string]float64)
	
	// Document frequency
	df := make(map[string]int)
	totalDocs := len(documents)
	
	vocabIndex := 0
	for _, doc := range documents {
		// Tokenize document
		tokens := s.tokenize(doc.Content)
		seen := make(map[string]bool)
		
		for _, token := range tokens {
			if _, exists := s.vocabulary[token]; !exists {
				s.vocabulary[token] = vocabIndex
				vocabIndex++
			}
			
			// Count document frequency
			if !seen[token] {
				df[token]++
				seen[token] = true
			}
		}
	}
	
	// Calculate IDF
	for token, freq := range df {
		s.idf[token] = math.Log(float64(totalDocs) / float64(freq))
	}
}

// GetEmbedding returns TF-IDF vector for text
func (s *SimpleEmbeddingService) GetEmbedding(text string) ([]float32, error) {
	tokens := s.tokenize(text)
	
	// Create TF-IDF vector
	vector := make([]float32, len(s.vocabulary))
	
	// Calculate term frequency
	tf := make(map[string]int)
	for _, token := range tokens {
		tf[token]++
	}
	
	// Calculate TF-IDF
	for token, freq := range tf {
		if idx, exists := s.vocabulary[token]; exists {
			tfValue := float64(freq) / float64(len(tokens))
			idfValue := s.idf[token]
			vector[idx] = float32(tfValue * idfValue)
		}
	}
	
	return vector, nil
}

// CosineSimilarity calculates cosine similarity between two vectors
func (s *SimpleEmbeddingService) CosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}
	
	var dotProduct, normA, normB float32
	
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	
	if normA == 0 || normB == 0 {
		return 0
	}
	
	return dotProduct / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
}

// tokenize splits text into tokens (simple word-based tokenization)
func (s *SimpleEmbeddingService) tokenize(text string) []string {
	// Convert to lowercase
	text = strings.ToLower(text)
	
	// Replace punctuation with spaces
	replacer := strings.NewReplacer(
		".", " ", ",", " ", ";", " ", ":", " ",
		"!", " ", "?", " ", "(", " ", ")", " ",
		"[", " ", "]", " ", "{", " ", "}", " ",
		"\"", " ", "'", " ", "-", " ", "_", " ",
		"\n", " ", "\t", " ", "\r", " ",
	)
	text = replacer.Replace(text)
	
	// Split by whitespace
	words := strings.Fields(text)
	
	// Filter out short words and numbers
	var tokens []string
	for _, word := range words {
		if len(word) > 2 && !isNumber(word) {
			tokens = append(tokens, word)
		}
	}
	
	return tokens
}

// isNumber checks if a string is a number
func isNumber(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// DocumentWithScore represents a document with its relevance score
type DocumentWithScore struct {
	Document Document
	Score    float32
}

// VectorKnowledgeService extends KnowledgeService with vector search
type VectorKnowledgeService struct {
	*KnowledgeService
	embedding  *SimpleEmbeddingService
	docVectors map[string][]float32
}

// NewVectorKnowledgeService creates a new vector-enabled knowledge service
func NewVectorKnowledgeService(logger *logrus.Logger) *VectorKnowledgeService {
	ks := NewKnowledgeService(logger).(*KnowledgeService)
	return &VectorKnowledgeService{
		KnowledgeService: ks,
		embedding:        NewSimpleEmbeddingService(),
		docVectors:       make(map[string][]float32),
	}
}

// LoadKnowledgeBase loads documents and builds embeddings
func (v *VectorKnowledgeService) LoadKnowledgeBase(ctx context.Context, dir string) error {
	// Load documents
	if err := v.KnowledgeService.LoadKnowledgeBase(ctx, dir); err != nil {
		return err
	}
	
	// Build embeddings
	docs := v.GetAllDocuments()
	v.embedding.BuildVocabulary(docs)
	
	// Create document vectors
	v.docVectors = make(map[string][]float32)
	for _, doc := range docs {
		vector, err := v.embedding.GetEmbedding(doc.Content)
		if err != nil {
			v.logger.WithError(err).WithField("doc", doc.ID).Warn("Failed to create embedding")
			continue
		}
		v.docVectors[doc.ID] = vector
	}
	
	v.logger.WithField("vectors", len(v.docVectors)).Info("Document vectors created")
	return nil
}

// VectorSearch performs semantic search using embeddings
func (v *VectorKnowledgeService) VectorSearch(ctx context.Context, query string, limit int) ([]DocumentWithScore, error) {
	// Get query embedding
	queryVector, err := v.embedding.GetEmbedding(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get query embedding: %w", err)
	}
	
	// Calculate similarities
	var results []DocumentWithScore
	
	v.documentsRW.RLock()
	defer v.documentsRW.RUnlock()
	
	for docID, docVector := range v.docVectors {
		if doc, exists := v.documents[docID]; exists {
			score := v.embedding.CosineSimilarity(queryVector, docVector)
			if score > 0.1 { // Threshold for relevance
				results = append(results, DocumentWithScore{
					Document: *doc,
					Score:    score,
				})
			}
		}
	}
	
	// Sort by score
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	
	// Limit results
	if len(results) > limit {
		results = results[:limit]
	}
	
	return results, nil
}