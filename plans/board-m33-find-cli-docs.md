# M33: find CLI - Document Commands

## Overview

Add `board find estimate/invoice/order/delivery/receipt` CLI commands that expose the find service's document search capabilities.

## Implementation

All 5 document commands follow the identical pattern established by M32 (find_client.go, find_project.go):

1. Flags: `--id`, `--client-name`, `--project-name`, `--text`, `--status`
2. Validation: at least one flag must be specified
3. Query construction: flags -> FindXxxQuery struct -> svc.FindXxx() -> output.Write()
4. Helper reuse: findServiceFromCmd(), readOptionsFromCmd(), prettyFromCmd()

## Files

- [x] `internal/cli/find_estimate.go` — NewFindEstimateCmd
- [x] `internal/cli/find_invoice.go` — NewFindInvoiceCmd
- [x] `internal/cli/find_order.go` — NewFindOrderCmd
- [x] `internal/cli/find_delivery.go` — NewFindDeliveryCmd
- [x] `internal/cli/find_receipt.go` — NewFindReceiptCmd
- [x] `internal/cli/find.go` — Register 5 new subcommands in NewFindCmd()
- [x] `internal/cli/find_test.go` — Tests for all 5 commands (Use, flags, validation)

## Testing

- `go vet ./...` — PASS
- `go test ./...` — PASS (all 11 packages)
