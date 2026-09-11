package sqlite

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/saintedlama/archscout/codegraph"
	sqlite "modernc.org/sqlite"
)

func init() {
	_ = sqlite.RegisterScalarFunction(
		"vec_distance_cosine",
		2,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			if len(args) < 2 || args[0] == nil || args[1] == nil {
				return 1.0, nil
			}
			v1 := toFloat32Slice(args[0])
			v2 := toFloat32Slice(args[1])
			if v1 == nil || v2 == nil {
				return 1.0, nil
			}
			return CosineDistance(v1, v2), nil
		},
	)

	_ = sqlite.RegisterScalarFunction(
		"vec_distance_l2",
		2,
		func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			if len(args) < 2 || args[0] == nil || args[1] == nil {
				return 0.0, nil
			}
			v1 := toFloat32Slice(args[0])
			v2 := toFloat32Slice(args[1])
			if v1 == nil || v2 == nil {
				return 0.0, nil
			}
			return L2Distance(v1, v2), nil
		},
	)
}

// EmbeddingFunc calculates vector embeddings for a batch of input texts.
type EmbeddingFunc func(ctx context.Context, texts []string) ([][]float32, error)

// Options configures the SQLite export.
type Options struct {
	EmbeddingFunc EmbeddingFunc
	Dimensions    int
}

// Option modifies Options.
type Option func(*Options)

// WithEmbeddingFunc configures a vector embedding generator.
func WithEmbeddingFunc(fn EmbeddingFunc, dimensions int) Option {
	return func(o *Options) {
		o.EmbeddingFunc = fn
		o.Dimensions = dimensions
	}
}

// Export serializes a CodeGraph into a SQLite database with FTS5 and vector similarity search.
func Export(ctx context.Context, g *codegraph.Graph, dbPath string, opts ...Option) error {
	if g == nil {
		return fmt.Errorf("codegraph is nil")
	}

	var cfg Options
	for _, opt := range opts {
		opt(&cfg)
	}

	dsn := dbPath
	if !strings.HasPrefix(dsn, "file:") && dsn != ":memory:" {
		dsn = fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)", dbPath)
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("open sqlite database: %w", err)
	}
	defer db.Close()

	if err := initSchema(db, cfg.Dimensions); err != nil {
		return fmt.Errorf("init sqlite schema: %w", err)
	}

	if err := insertGraph(ctx, db, g, cfg); err != nil {
		return fmt.Errorf("insert graph data: %w", err)
	}

	return nil
}

func initSchema(db *sql.DB, vectorDim int) error {
	schema := `
	CREATE TABLE IF NOT EXISTS nodes (
		rowid INTEGER PRIMARY KEY AUTOINCREMENT,
		id TEXT UNIQUE NOT NULL,
		kind TEXT NOT NULL,
		name TEXT NOT NULL,
		qname TEXT,
		package_id TEXT,
		parent_id TEXT,
		within_workspace INTEGER NOT NULL,
		filename TEXT,
		line INTEGER,
		col INTEGER
	);

	CREATE TABLE IF NOT EXISTS edges (
		source_id TEXT NOT NULL,
		target_id TEXT NOT NULL,
		kind TEXT NOT NULL,
		filename TEXT,
		line INTEGER,
		col INTEGER,
		PRIMARY KEY (source_id, target_id, kind)
	);

	CREATE INDEX IF NOT EXISTS idx_edges_source ON edges(source_id, kind);
	CREATE INDEX IF NOT EXISTS idx_edges_target ON edges(target_id, kind);
	CREATE INDEX IF NOT EXISTS idx_nodes_kind ON nodes(kind);
	CREATE INDEX IF NOT EXISTS idx_nodes_package ON nodes(package_id);
	CREATE INDEX IF NOT EXISTS idx_nodes_qname ON nodes(qname);

	CREATE VIRTUAL TABLE IF NOT EXISTS nodes_fts USING fts5(
		name,
		qname,
		package_id,
		content='nodes',
		content_rowid='rowid'
	);

	CREATE TABLE IF NOT EXISTS node_embeddings (
		node_id TEXT PRIMARY KEY,
		embedding BLOB NOT NULL,
		dimensions INTEGER NOT NULL,
		FOREIGN KEY(node_id) REFERENCES nodes(id)
	);

	CREATE INDEX IF NOT EXISTS idx_embeddings_node ON node_embeddings(node_id);
	`
	_, err := db.Exec(schema)
	return err
}

func insertGraph(ctx context.Context, db *sql.DB, g *codegraph.Graph, cfg Options) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	nodeStmt, err := tx.PrepareContext(ctx, `
		INSERT OR REPLACE INTO nodes(id, kind, name, qname, package_id, parent_id, within_workspace, filename, line, col)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer nodeStmt.Close()

	ftsStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO nodes_fts(rowid, name, qname, package_id)
		VALUES (?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer ftsStmt.Close()

	var insertedNodes []codegraph.Node

	for _, n := range g.Nodes() {
		within := 0
		if n.WithinWorkspace {
			within = 1
		}
		res, err := nodeStmt.ExecContext(ctx,
			n.ID,
			string(n.Kind),
			n.Name,
			n.QName,
			n.PackageID,
			n.ParentID,
			within,
			n.Ref.Filename,
			n.Ref.Line,
			n.Ref.Column,
		)
		if err != nil {
			return fmt.Errorf("insert node %s: %w", n.ID, err)
		}
		rowID, _ := res.LastInsertId()
		insertedNodes = append(insertedNodes, n)

		if _, err := ftsStmt.ExecContext(ctx, rowID, n.Name, n.QName, n.PackageID); err != nil {
			return fmt.Errorf("insert fts %s: %w", n.ID, err)
		}
	}

	edgeStmt, err := tx.PrepareContext(ctx, `
		INSERT OR REPLACE INTO edges(source_id, target_id, kind, filename, line, col)
		VALUES (?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer edgeStmt.Close()

	for _, e := range g.Edges() {
		if _, err := edgeStmt.ExecContext(ctx,
			e.Source,
			e.Target,
			string(e.Kind),
			e.Ref.Filename,
			e.Ref.Line,
			e.Ref.Column,
		); err != nil {
			return fmt.Errorf("insert edge %s -> %s: %w", e.Source, e.Target, err)
		}
	}

	// Calculate and insert embeddings if configured
	if cfg.EmbeddingFunc != nil && cfg.Dimensions > 0 && len(insertedNodes) > 0 {
		texts := make([]string, len(insertedNodes))
		for i, n := range insertedNodes {
			text := n.Name
			if n.QName != "" {
				text = n.QName
			}
			texts[i] = fmt.Sprintf("[%s] %s (package: %s)", n.Kind, text, n.PackageID)
		}

		vectors, err := cfg.EmbeddingFunc(ctx, texts)
		if err != nil {
			return fmt.Errorf("compute embeddings: %w", err)
		}

		vecStmt, err := tx.PrepareContext(ctx, `
			INSERT OR REPLACE INTO node_embeddings(node_id, embedding, dimensions)
			VALUES (?, ?, ?)
		`)
		if err != nil {
			return err
		}
		defer vecStmt.Close()

		for i, v := range vectors {
			if len(v) != cfg.Dimensions {
				continue
			}
			blob := SerializeVector(v)
			if _, err := vecStmt.ExecContext(ctx, insertedNodes[i].ID, blob, cfg.Dimensions); err != nil {
				return fmt.Errorf("insert embedding %s: %w", insertedNodes[i].ID, err)
			}
		}
	}

	return tx.Commit()
}

// SerializeVector converts a float32 slice to little-endian binary format.
func SerializeVector(v []float32) []byte {
	buf := make([]byte, len(v)*4)
	for i, f := range v {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(f))
	}
	return buf
}

// DeserializeVector decodes a little-endian binary blob to float32 slice.
func DeserializeVector(b []byte) []float32 {
	if len(b)%4 != 0 {
		return nil
	}
	v := make([]float32, len(b)/4)
	for i := range v {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return v
}

func toFloat32Slice(val any) []float32 {
	switch v := val.(type) {
	case []byte:
		return DeserializeVector(v)
	case string:
		var nums []float32
		if err := json.Unmarshal([]byte(v), &nums); err == nil {
			return nums
		}
	}
	return nil
}

// CosineDistance calculates cosine distance (1.0 - cosine_similarity).
func CosineDistance(v1, v2 []float32) float64 {
	if len(v1) != len(v2) || len(v1) == 0 {
		return 1.0
	}
	var dot, norm1, norm2 float64
	for i := range v1 {
		a := float64(v1[i])
		b := float64(v2[i])
		dot += a * b
		norm1 += a * a
		norm2 += b * b
	}
	if norm1 == 0 || norm2 == 0 {
		return 1.0
	}
	sim := dot / (math.Sqrt(norm1) * math.Sqrt(norm2))
	if sim > 1.0 {
		sim = 1.0
	} else if sim < -1.0 {
		sim = -1.0
	}
	return 1.0 - sim
}

// L2Distance calculates Euclidean L2 distance.
func L2Distance(v1, v2 []float32) float64 {
	if len(v1) != len(v2) || len(v1) == 0 {
		return 0.0
	}
	var sum float64
	for i := range v1 {
		diff := float64(v1[i]) - float64(v2[i])
		sum += diff * diff
	}
	return math.Sqrt(sum)
}

// Store provides query helpers over an exported ArchScout SQLite database.
type Store struct {
	db *sql.DB
}

// Open opens an existing ArchScout SQLite database.
func Open(dbPath string) (*Store, error) {
	dsn := dbPath
	if !strings.HasPrefix(dsn, "file:") && dsn != ":memory:" {
		dsn = fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)", dbPath)
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

// Close closes the underlying database.
func (s *Store) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

// DB returns the underlying *sql.DB for running custom agent queries.
func (s *Store) DB() *sql.DB {
	return s.db
}

// VectorMatch represents a node matched via vector similarity.
type VectorMatch struct {
	Node     codegraph.Node
	Distance float64
}

// SearchVector queries the nearest neighbor nodes using exact cosine distance.
func (s *Store) SearchVector(ctx context.Context, vector []float32, limit int) ([]VectorMatch, error) {
	queryBlob := SerializeVector(vector)
	query := `
		SELECT n.id, n.kind, n.name, n.qname, n.package_id, n.parent_id, n.within_workspace,
		       n.filename, n.line, n.col, vec_distance_cosine(e.embedding, ?) AS distance
		FROM node_embeddings e
		JOIN nodes n ON n.id = e.node_id
		ORDER BY distance ASC
		LIMIT ?;
	`
	rows, err := s.db.QueryContext(ctx, query, queryBlob, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var matches []VectorMatch
	for rows.Next() {
		var n codegraph.Node
		var kindStr string
		var within int
		var fn sql.NullString
		var line, col sql.NullInt64
		var dist float64

		if err := rows.Scan(
			&n.ID, &kindStr, &n.Name, &n.QName, &n.PackageID, &n.ParentID, &within,
			&fn, &line, &col, &dist,
		); err != nil {
			return nil, err
		}
		n.Kind = codegraph.NodeKind(kindStr)
		n.WithinWorkspace = within == 1
		if fn.Valid {
			n.Ref.Filename = fn.String
			n.Ref.Line = int(line.Int64)
			n.Ref.Column = int(col.Int64)
		}
		matches = append(matches, VectorMatch{Node: n, Distance: dist})
	}
	return matches, nil
}

// SearchFTS performs a full-text search across node names, qnames, and package IDs.
func (s *Store) SearchFTS(ctx context.Context, query string, limit int) ([]codegraph.Node, error) {
	sqlQuery := `
		SELECT n.id, n.kind, n.name, n.qname, n.package_id, n.parent_id, n.within_workspace,
		       n.filename, n.line, n.col
		FROM nodes_fts f
		JOIN nodes n ON n.rowid = f.rowid
		WHERE nodes_fts MATCH ?
		LIMIT ?;
	`
	rows, err := s.db.QueryContext(ctx, sqlQuery, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []codegraph.Node
	for rows.Next() {
		var n codegraph.Node
		var kindStr string
		var within int
		var fn sql.NullString
		var line, col sql.NullInt64

		if err := rows.Scan(
			&n.ID, &kindStr, &n.Name, &n.QName, &n.PackageID, &n.ParentID, &within,
			&fn, &line, &col,
		); err != nil {
			return nil, err
		}
		n.Kind = codegraph.NodeKind(kindStr)
		n.WithinWorkspace = within == 1
		if fn.Valid {
			n.Ref.Filename = fn.String
			n.Ref.Line = int(line.Int64)
			n.Ref.Column = int(col.Int64)
		}
		result = append(result, n)
	}
	return result, nil
}

// GetNode retrieves a single node by its ID, qualified name, or package ID. Returns nil if not found.
func (s *Store) GetNode(ctx context.Context, id string) (*codegraph.Node, error) {
	query := `
		SELECT id, kind, name, qname, package_id, parent_id, within_workspace,
		       filename, line, col
		FROM nodes
		WHERE id = ? OR qname = ? OR package_id = ? OR name = ?
		   OR id = 'package:' || ? OR id = 'function:' || ? OR id = 'type:' || ?
		ORDER BY (id = ?) DESC, (qname = ?) DESC
		LIMIT 1;
	`
	row := s.db.QueryRowContext(ctx, query, id, id, id, id, id, id, id, id, id)
	var n codegraph.Node
	var kindStr string
	var within int
	var fn sql.NullString
	var line, col sql.NullInt64

	if err := row.Scan(
		&n.ID, &kindStr, &n.Name, &n.QName, &n.PackageID, &n.ParentID, &within,
		&fn, &line, &col,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	n.Kind = codegraph.NodeKind(kindStr)
	n.WithinWorkspace = within == 1
	if fn.Valid {
		n.Ref.Filename = fn.String
		n.Ref.Line = int(line.Int64)
		n.Ref.Column = int(col.Int64)
	}
	return &n, nil
}

// GetImplementers returns the IDs of concrete types implementing the interface ifaceID.
func (s *Store) GetImplementers(ctx context.Context, ifaceID string) ([]string, error) {
	query := `
		SELECT source_id
		FROM edges
		WHERE (target_id = ? OR target_id = 'type:' || ?) AND kind = 'implements'
		ORDER BY source_id ASC;
	`
	rows, err := s.db.QueryContext(ctx, query, ifaceID, ifaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var implementers []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		implementers = append(implementers, id)
	}
	return implementers, nil
}

// TraceCallers performs a multi-hop recursive CTE to find all callers of targetNodeID.
func (s *Store) TraceCallers(ctx context.Context, targetNodeID string, maxDepth int) ([]string, error) {
	if maxDepth <= 0 {
		maxDepth = 5
	}
	query := `
		WITH RECURSIVE callers AS (
			SELECT source_id, target_id, 1 AS depth
			FROM edges
			WHERE (target_id = ? OR target_id = 'function:' || ? OR target_id = 'type:' || ?) AND kind = 'calls'
			UNION ALL
			SELECT e.source_id, e.target_id, c.depth + 1
			FROM edges e
			JOIN callers c ON e.target_id = c.source_id
			WHERE e.kind = 'calls' AND c.depth < ?
		)
		SELECT DISTINCT source_id FROM callers;
	`
	rows, err := s.db.QueryContext(ctx, query, targetNodeID, targetNodeID, targetNodeID, maxDepth)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var callerIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		callerIDs = append(callerIDs, id)
	}
	return callerIDs, nil
}

// TraceDependencies performs a multi-hop recursive CTE to find all dependencies reachable from sourceNodeID.
func (s *Store) TraceDependencies(ctx context.Context, sourceNodeID string, maxDepth int) ([]string, error) {
	if maxDepth <= 0 {
		maxDepth = 5
	}
	query := `
		WITH RECURSIVE deps AS (
			SELECT source_id, target_id, 1 AS depth
			FROM edges
			WHERE (source_id = ? OR source_id = 'package:' || ? OR source_id = 'function:' || ? OR source_id = 'type:' || ?)
			  AND kind IN ('calls', 'imports', 'depends_on')
			UNION ALL
			SELECT e.source_id, e.target_id, d.depth + 1
			FROM edges e
			JOIN deps d ON e.source_id = d.target_id
			WHERE e.kind IN ('calls', 'imports', 'depends_on') AND d.depth < ?
		)
		SELECT DISTINCT target_id FROM deps;
	`
	rows, err := s.db.QueryContext(ctx, query, sourceNodeID, sourceNodeID, sourceNodeID, sourceNodeID, maxDepth)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var depIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		depIDs = append(depIDs, id)
	}
	return depIDs, nil
}

// HybridExpansion represents an entry point matched by vector similarity alongside its callers.
type HybridExpansion struct {
	MatchedNodeID string
	Distance      float64
	CallerIDs     []string
}

// HybridSearch finds top-K semantically closest nodes and simultaneously traverses their callers using a single SQL CTE query.
func (s *Store) HybridSearch(ctx context.Context, queryVector []float32, topK int, callerDepth int) ([]HybridExpansion, error) {
	if callerDepth <= 0 {
		callerDepth = 2
	}
	queryBlob := SerializeVector(queryVector)
	query := `
		WITH top_seeds AS (
			SELECT e.node_id, vec_distance_cosine(e.embedding, ?) AS distance
			FROM node_embeddings e
			ORDER BY distance ASC
			LIMIT ?
		),
		callers AS (
			SELECT s.node_id AS seed_id, s.distance, ed.source_id AS caller_id, 1 AS depth
			FROM top_seeds s
			LEFT JOIN edges ed ON ed.target_id = s.node_id AND ed.kind = 'calls'
			UNION ALL
			SELECT c.seed_id, c.distance, ed.source_id, c.depth + 1
			FROM edges ed
			JOIN callers c ON ed.target_id = c.caller_id
			WHERE ed.kind = 'calls' AND c.depth < ?
		)
		SELECT seed_id, distance, caller_id FROM callers;
	`
	rows, err := s.db.QueryContext(ctx, query, queryBlob, topK, callerDepth)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	expMap := make(map[string]*HybridExpansion)
	var order []string

	for rows.Next() {
		var seedID string
		var dist float64
		var callerID sql.NullString

		if err := rows.Scan(&seedID, &dist, &callerID); err != nil {
			return nil, err
		}

		entry, ok := expMap[seedID]
		if !ok {
			entry = &HybridExpansion{
				MatchedNodeID: seedID,
				Distance:      dist,
			}
			expMap[seedID] = entry
			order = append(order, seedID)
		}
		if callerID.Valid && callerID.String != "" {
			entry.CallerIDs = append(entry.CallerIDs, callerID.String)
		}
	}

	result := make([]HybridExpansion, len(order))
	for i, id := range order {
		result[i] = *expMap[id]
	}
	return result, nil
}
