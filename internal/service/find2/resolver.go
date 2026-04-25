package find2

import (
	"context"
	"log/slog"

	"github.com/youyo/board/internal/boardapi"
	"github.com/youyo/board/internal/repository"
	"golang.org/x/sync/errgroup"
)

// resolveClientAndProject は clientID / projectID を並列で解決し enrichment 結果を返す。
// エラーは非致命として swallow し slog.Warn を出力する（N02 §4.3）。
// 両 ID が 0 の場合は no-op で (nil, nil) を返す（T17）。
func (s *Service) resolveClientAndProject(
	ctx context.Context,
	clientID, projectID int,
	opts repository.ReadOptions,
) (*boardapi.ClientEntity, *boardapi.ProjectEntity) {
	if clientID == 0 && projectID == 0 {
		return nil, nil
	}
	var client *boardapi.ClientEntity
	var project *boardapi.ProjectEntity
	g, gctx := errgroup.WithContext(ctx)
	if clientID != 0 {
		g.Go(func() error {
			c, err := s.clients.GetByID(gctx, clientID, opts)
			if err != nil {
				slog.Warn("find2.resolveClientAndProject: client enrichment failed",
					"client_id", clientID, "error", err)
				return nil // 非致命: error は swallow
			}
			client = c
			return nil
		})
	}
	if projectID != 0 {
		g.Go(func() error {
			p, err := s.projects.GetByID(gctx, projectID, opts)
			if err != nil {
				slog.Warn("find2.resolveClientAndProject: project enrichment failed",
					"project_id", projectID, "error", err)
				return nil // 非致命: error は swallow
			}
			project = p
			return nil
		})
	}
	_ = g.Wait()
	return client, project
}

// resolveVendorAndProject は vendorID / projectID を並列で解決し enrichment 結果を返す。
// エラーは非致命として swallow し slog.Warn を出力する（N02 §4.3）。
// 両 ID が 0 の場合は no-op で (nil, nil) を返す。
func (s *Service) resolveVendorAndProject(
	ctx context.Context,
	vendorID, projectID int,
	opts repository.ReadOptions,
) (*boardapi.VendorEntity, *boardapi.ProjectEntity) {
	if vendorID == 0 && projectID == 0 {
		return nil, nil
	}
	var vendor *boardapi.VendorEntity
	var project *boardapi.ProjectEntity
	g, gctx := errgroup.WithContext(ctx)
	if vendorID != 0 {
		g.Go(func() error {
			v, err := s.vendors.GetByID(gctx, vendorID, opts)
			if err != nil {
				slog.Warn("find2.resolveVendorAndProject: vendor enrichment failed",
					"vendor_id", vendorID, "error", err)
				return nil // 非致命: error は swallow
			}
			vendor = v
			return nil
		})
	}
	if projectID != 0 {
		g.Go(func() error {
			p, err := s.projects.GetByID(gctx, projectID, opts)
			if err != nil {
				slog.Warn("find2.resolveVendorAndProject: project enrichment failed",
					"project_id", projectID, "error", err)
				return nil // 非致命: error は swallow
			}
			project = p
			return nil
		})
	}
	_ = g.Wait()
	return vendor, project
}
