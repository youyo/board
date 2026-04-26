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

// resolveProjectClient は Project に紐づく Client を取得し ProjectResult を返す。
// nested Client が nil または ClientID == 0 の場合は enrichment をスキップし、
// Client=nil で Result を返す。
//
// enrichment ポリシー（resolver.go top doc の discriminator に従う）:
//   - 補助情報（Client）取得失敗は non-fatal（slog.Warn + Client=nil で Result 返却）
//   - ctx cancel / deadline 由来も同様の扱い
//
// 並列化は不要（補助情報が 1 件のみ）。N06 Document 4 種では再び
// resolveClientAndProject（errgroup 並列版）を使用する。
func (s *Service) resolveProjectClient(ctx context.Context, project boardapi.ProjectEntity, opts repository.ReadOptions) ProjectResult {
	cid := projectClientIDPtr(&project)
	if cid == 0 {
		return ProjectResult{Project: project, Client: nil}
	}
	c, err := s.clients.GetByID(ctx, cid, opts)
	if err != nil {
		slog.Warn("find2.resolveProjectClient: client enrichment failed",
			"project_id", project.ID, "client_id", cid, "error", err)
		return ProjectResult{Project: project, Client: nil}
	}
	return ProjectResult{Project: project, Client: c}
}

// resolveClientDetails は単一クライアントの branches と contacts を errgroup で 2 並列取得し、
// ClientResult を返す。
//
// # enrichment ポリシー（N02 §4.3 / N03 resolveClientAndProject と整合）
//
//   - 「Result の必須フィールドを満たすための主体取得」 → fail-fast（呼び出し元に error 伝播）
//   - 「主体に対する補助情報（branches / contacts 等）の enrichment」 → non-fatal（slog.Warn + nil/空で Result 返却）
//
// ctx cancel / deadline 由来の error も non-fatal として同様に扱う（呼び出し元への error 伝播なし）。
func (s *Service) resolveClientDetails(ctx context.Context, client boardapi.ClientEntity, opts repository.ReadOptions) ClientResult {
	var branches []boardapi.ClientBranchEntity
	var contacts []boardapi.ContactEntity
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		b, err := s.clientBranches.Search(gctx, boardapi.ClientBranchListOptions{ClientIDEq: client.ID}, opts)
		if err != nil {
			slog.Warn("find2.resolveClientDetails: branches enrichment failed",
				"client_id", client.ID, "error", err)
			return nil // non-fatal: error は swallow
		}
		branches = b
		return nil
	})
	g.Go(func() error {
		c, err := s.contacts.Search(gctx, boardapi.ContactListOptions{ClientIDEq: client.ID}, opts)
		if err != nil {
			slog.Warn("find2.resolveClientDetails: contacts enrichment failed",
				"client_id", client.ID, "error", err)
			return nil // non-fatal: error は swallow
		}
		contacts = c
		return nil
	})
	_ = g.Wait()
	return ClientResult{Client: client, Branches: branches, Contacts: contacts}
}

// resolveVendorDetails は単一仕入先の branches と contacts を errgroup で 2 並列取得し、
// VendorResult を返す。
//
// enrichment ポリシーは resolveClientDetails と同一（non-fatal + slog.Warn）。
//
// 注意: VendorBranch / VendorContact のフィルタキーは PayeeIDEq（ClientIDEq ではない）。
// BOARD API の /v1/payees エンドポイント仕様に基づく（リスク R2）。
func (s *Service) resolveVendorDetails(ctx context.Context, vendor boardapi.VendorEntity, opts repository.ReadOptions) VendorResult {
	var branches []boardapi.VendorBranchEntity
	var contacts []boardapi.VendorContactEntity
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		b, err := s.vendorBranches.Search(gctx, boardapi.VendorBranchListOptions{PayeeIDEq: vendor.ID}, opts)
		if err != nil {
			slog.Warn("find2.resolveVendorDetails: branches enrichment failed",
				"vendor_id", vendor.ID, "error", err)
			return nil // non-fatal: error は swallow
		}
		branches = b
		return nil
	})
	g.Go(func() error {
		c, err := s.vendorContacts.Search(gctx, boardapi.VendorContactListOptions{PayeeIDEq: vendor.ID}, opts)
		if err != nil {
			slog.Warn("find2.resolveVendorDetails: contacts enrichment failed",
				"vendor_id", vendor.ID, "error", err)
			return nil // non-fatal: error は swallow
		}
		contacts = c
		return nil
	})
	_ = g.Wait()
	return VendorResult{Vendor: vendor, Branches: branches, Contacts: contacts}
}
