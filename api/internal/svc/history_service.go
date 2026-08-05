package svc

import (
	"context"
	"fmt"
	"time"

	"cscan/internal/model"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// HistoryService manages historical scan result versions
type HistoryService struct {
	db *mongo.Database
}

// NewHistoryService creates a new HistoryService
func NewHistoryService(db *mongo.Database) *HistoryService {
	return &HistoryService{
		db: db,
	}
}

// ==================== Request/Response Types ====================

// ArchiveResultsReq represents a request to archive current scan results
type ArchiveResultsReq struct {
	WorkspaceId string
	TargetId    string
	Authority   string
	Host        string
	Port        int
	ArchiveTime time.Time
}

// GetResultHistoryReq represents a request to retrieve historical versions
type GetResultHistoryReq struct {
	WorkspaceId string
	Authority   string
	Host        string
	Port        int
	StartTime   time.Time
	EndTime     time.Time
}

// GetResultHistoryResp represents the response with historical versions
type GetResultHistoryResp struct {
	Versions []HistoricalVersion
}

// HistoricalVersion represents a single historical scan version
type HistoricalVersion struct {
	VersionId      string
	ScanTimestamp  time.Time
	DirScanCount   int64
	VulnScanCount  int64
	ChangesSummary string
}

// CompareVersionsReq represents a request to compare two versions
type CompareVersionsReq struct {
	WorkspaceId string
	VersionId1  string
	VersionId2  string
}

// CompareVersionsResp represents the response with version comparison
type CompareVersionsResp struct {
	Version1         HistoricalVersion
	Version2         HistoricalVersion
	DirScansAdded    int64
	DirScansRemoved  int64
	VulnsAdded       int64
	VulnsRemoved     int64
	ComparisonDetail string
}

// ==================== Service Methods ====================

// ArchiveCurrentResults moves existing scan results to the asset_history collection
// This method ensures atomicity by using MongoDB transactions
func (s *HistoryService) ArchiveCurrentResults(ctx context.Context, req *ArchiveResultsReq) error {
	// Validate required parameters
	if req.WorkspaceId == "" {
		return fmt.Errorf("workspace_id is required")
	}
	if req.Host == "" {
		return fmt.Errorf("host is required")
	}
	if req.Port == 0 {
		return fmt.Errorf("port is required")
	}

	// Set default archive time if not provided
	if req.ArchiveTime.IsZero() {
		req.ArchiveTime = time.Now()
	}

	// Start a MongoDB session for transaction support
	session, err := s.db.Client().StartSession()
	if err != nil {
		return fmt.Errorf("failed to start session: %w", err)
	}
	defer session.EndSession(ctx)

	// Execute the archival operation within a transaction
	_, err = session.WithTransaction(ctx, func(sessCtx mongo.SessionContext) (interface{}, error) {
		// Step 1: Query existing directory scan results
		dirScanModel := model.NewDirScanResultModel(s.db)
		dirFilter := bson.M{
			"workspace_id": req.WorkspaceId,
			"host":         req.Host,
			"port":         req.Port,
		}
		if req.Authority != "" {
			dirFilter["authority"] = req.Authority
		}

		dirResults, err := dirScanModel.FindByFilter(sessCtx, dirFilter, 0, 0)
		if err != nil {
			return nil, fmt.Errorf("failed to query directory scan results: %w", err)
		}

		// Step 2: Query existing vulnerability scan results
		scanResultModel := model.NewScanResultModel(s.db, req.WorkspaceId)
		vulnFilter := bson.M{
			"host": req.Host,
			"port": req.Port,
		}
		if req.Authority != "" {
			vulnFilter["authority"] = req.Authority
		}

		vulnResults, err := scanResultModel.FindWithSort(sessCtx, vulnFilter, 0, 0, "create_time", -1)
		if err != nil {
			return nil, fmt.Errorf("failed to query vulnerability scan results: %w", err)
		}

		// If no results exist, nothing to archive
		if len(dirResults) == 0 && len(vulnResults) == 0 {
			return nil, nil
		}

		// Step 3: Determine the scan timestamp (use the most recent)
		var scanTimestamp time.Time
		if len(dirResults) > 0 && !dirResults[0].ScanTime.IsZero() {
			scanTimestamp = dirResults[0].ScanTime
		}
		if len(vulnResults) > 0 && !vulnResults[0].ScanTime.IsZero() {
			if scanTimestamp.IsZero() || vulnResults[0].ScanTime.After(scanTimestamp) {
				scanTimestamp = vulnResults[0].ScanTime
			}
		}
		// If no scan_time is set, use created or current time
		if scanTimestamp.IsZero() {
			if len(dirResults) > 0 && !dirResults[0].CreateTime.IsZero() {
				scanTimestamp = dirResults[0].CreateTime
			} else if len(vulnResults) > 0 && !vulnResults[0].Created.IsZero() {
				scanTimestamp = vulnResults[0].Created
			} else {
				scanTimestamp = req.ArchiveTime
			}
		}

		// Step 4: Create historical record
		historyModel := model.NewScanResultHistoryModel(s.db, req.WorkspaceId)
		versionId := uuid.New().String()

		changesSummary := fmt.Sprintf("Archived %d directory scans and %d vulnerability scans",
			len(dirResults), len(vulnResults))

		historyRecord := &model.ScanResultHistory{
			WorkspaceId:     req.WorkspaceId,
			AssetId:         req.TargetId,
			Authority:       req.Authority,
			Host:            req.Host,
			Port:            req.Port,
			VersionId:       versionId,
			ScanTimestamp:   scanTimestamp,
			DirScanResults:  dirResults,
			VulnScanResults: vulnResults,
			ChangesSummary:  changesSummary,
			ArchivedAt:      req.ArchiveTime,
		}

		// Step 5: Insert historical record
		if err := historyModel.Insert(sessCtx, historyRecord); err != nil {
			return nil, fmt.Errorf("failed to insert historical record: %w", err)
		}

		// Step 6: Delete archived results from current collections
		// Note: We don't delete the results here because the design specifies
		// that new results will replace old ones. The archival is just for preservation.
		// The actual deletion/replacement happens when new scan results are saved.

		return versionId, nil
	})

	if err != nil {
		return fmt.Errorf("transaction failed: %w", err)
	}

	return nil
}

// GetResultHistory retrieves historical scan result versions within a time range
func (s *HistoryService) GetResultHistory(ctx context.Context, req *GetResultHistoryReq) (*GetResultHistoryResp, error) {
	// Validate required parameters
	if req.WorkspaceId == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if req.Host == "" {
		return nil, fmt.Errorf("host is required")
	}
	if req.Port == 0 {
		return nil, fmt.Errorf("port is required")
	}

	historyModel := model.NewScanResultHistoryModel(s.db, req.WorkspaceId)

	var histories []model.ScanResultHistory
	var err error

	// Query by time range if provided, otherwise get all versions
	if !req.StartTime.IsZero() && !req.EndTime.IsZero() {
		histories, err = historyModel.FindByTimeRange(ctx, req.WorkspaceId, req.Authority, req.Host, req.Port, req.StartTime, req.EndTime)
	} else {
		// Get all versions for this asset (limited to reasonable number)
		histories, err = historyModel.FindByAuthority(ctx, req.WorkspaceId, req.Authority, req.Host, req.Port, 100)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to query historical versions: %w", err)
	}

	// Convert to response format
	versions := make([]HistoricalVersion, 0, len(histories))
	for _, h := range histories {
		versions = append(versions, HistoricalVersion{
			VersionId:      h.VersionId,
			ScanTimestamp:  h.ScanTimestamp,
			DirScanCount:   int64(len(h.DirScanResults)),
			VulnScanCount:  int64(len(h.VulnScanResults)),
			ChangesSummary: h.ChangesSummary,
		})
	}

	return &GetResultHistoryResp{
		Versions: versions,
	}, nil
}

// CompareVersions compares two historical scan versions and shows differences
func (s *HistoryService) CompareVersions(ctx context.Context, req *CompareVersionsReq) (*CompareVersionsResp, error) {
	// Validate required parameters
	if req.WorkspaceId == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if req.VersionId1 == "" || req.VersionId2 == "" {
		return nil, fmt.Errorf("both version IDs are required")
	}

	historyModel := model.NewScanResultHistoryModel(s.db, req.WorkspaceId)

	// Fetch both versions
	version1, err := historyModel.FindByVersionId(ctx, req.WorkspaceId, req.VersionId1)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch version 1: %w", err)
	}
	if version1 == nil {
		return nil, fmt.Errorf("version 1 not found: %s", req.VersionId1)
	}

	version2, err := historyModel.FindByVersionId(ctx, req.WorkspaceId, req.VersionId2)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch version 2: %w", err)
	}
	if version2 == nil {
		return nil, fmt.Errorf("version 2 not found: %s", req.VersionId2)
	}

	// Calculate differences
	dirScansAdded := int64(len(version2.DirScanResults)) - int64(len(version1.DirScanResults))
	vulnsAdded := int64(len(version2.VulnScanResults)) - int64(len(version1.VulnScanResults))

	dirScansRemoved := int64(0)
	vulnsRemoved := int64(0)
	if dirScansAdded < 0 {
		dirScansRemoved = -dirScansAdded
		dirScansAdded = 0
	}
	if vulnsAdded < 0 {
		vulnsRemoved = -vulnsAdded
		vulnsAdded = 0
	}

	// Generate comparison detail
	comparisonDetail := fmt.Sprintf(
		"Version 1 (%s): %d dir scans, %d vulns\nVersion 2 (%s): %d dir scans, %d vulns\nChanges: %+d dir scans, %+d vulns",
		version1.ScanTimestamp.Format(time.RFC3339),
		len(version1.DirScanResults),
		len(version1.VulnScanResults),
		version2.ScanTimestamp.Format(time.RFC3339),
		len(version2.DirScanResults),
		len(version2.VulnScanResults),
		int64(len(version2.DirScanResults))-int64(len(version1.DirScanResults)),
		int64(len(version2.VulnScanResults))-int64(len(version1.VulnScanResults)),
	)

	return &CompareVersionsResp{
		Version1: HistoricalVersion{
			VersionId:      version1.VersionId,
			ScanTimestamp:  version1.ScanTimestamp,
			DirScanCount:   int64(len(version1.DirScanResults)),
			VulnScanCount:  int64(len(version1.VulnScanResults)),
			ChangesSummary: version1.ChangesSummary,
		},
		Version2: HistoricalVersion{
			VersionId:      version2.VersionId,
			ScanTimestamp:  version2.ScanTimestamp,
			DirScanCount:   int64(len(version2.DirScanResults)),
			VulnScanCount:  int64(len(version2.VulnScanResults)),
			ChangesSummary: version2.ChangesSummary,
		},
		DirScansAdded:    dirScansAdded,
		DirScansRemoved:  dirScansRemoved,
		VulnsAdded:       vulnsAdded,
		VulnsRemoved:     vulnsRemoved,
		ComparisonDetail: comparisonDetail,
	}, nil
}
