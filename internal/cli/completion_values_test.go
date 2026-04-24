package cli

import (
	"reflect"
	"testing"

	"github.com/spf13/cobra"
)

// ------------------------------------------------------------
// M58: 固定列挙候補テーブルの存在・内容テスト
// ------------------------------------------------------------

func TestResponseGroupCommon(t *testing.T) {
	want := []string{"small", "large"}
	if !reflect.DeepEqual(responseGroupCommon, want) {
		t.Errorf("responseGroupCommon = %v, want %v", responseGroupCommon, want)
	}
}

func TestResponseGroupProjectsList(t *testing.T) {
	want := []string{"small", "large", "estimate", "order", "delivery", "invoice", "receipt", "all"}
	if !reflect.DeepEqual(responseGroupProjectsList, want) {
		t.Errorf("responseGroupProjectsList = %v, want %v", responseGroupProjectsList, want)
	}
}

func TestResponseGroupProjectsGet(t *testing.T) {
	want := []string{"estimate", "order", "delivery", "invoice", "receipt", "all"}
	if !reflect.DeepEqual(responseGroupProjectsGet, want) {
		t.Errorf("responseGroupProjectsGet = %v, want %v", responseGroupProjectsGet, want)
	}
}

func TestOrderStatusMap(t *testing.T) {
	want := map[int]string{
		1: "見積中(高)",
		2: "見積中(中)",
		3: "見積中(低)",
		4: "受注確定",
		5: "受注済",
		8: "見積中(除)",
	}
	if !reflect.DeepEqual(orderStatusMap, want) {
		t.Errorf("orderStatusMap = %v, want %v", orderStatusMap, want)
	}
}

func TestDeliveryStatusMap(t *testing.T) {
	want := map[int]string{
		1: "未着手",
		2: "着手中",
		3: "納品済",
		4: "検収済",
	}
	if !reflect.DeepEqual(deliveryStatusMap, want) {
		t.Errorf("deliveryStatusMap = %v, want %v", deliveryStatusMap, want)
	}
}

func TestInvoiceTimingKbnMap(t *testing.T) {
	want := map[int]string{
		1: "一括請求",
		2: "定期請求",
	}
	if !reflect.DeepEqual(invoiceTimingKbnMap, want) {
		t.Errorf("invoiceTimingKbnMap = %v, want %v", invoiceTimingKbnMap, want)
	}
}

// ------------------------------------------------------------
// ヘルパ関数テスト
// ------------------------------------------------------------

func TestStaticCompletion(t *testing.T) {
	fn := staticCompletion([]string{"a", "b"})
	vals, dir := fn(nil, nil, "")
	if !reflect.DeepEqual(vals, []string{"a", "b"}) {
		t.Errorf("staticCompletion values = %v, want [a b]", vals)
	}
	if dir != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("staticCompletion directive = %v, want NoFileComp", dir)
	}
}

func TestStaticCompletion_IsDefensiveCopy(t *testing.T) {
	// staticCompletion が slice を保持していて外部から書き換えできないこと
	src := []string{"x", "y"}
	fn := staticCompletion(src)
	src[0] = "MUTATED"
	vals, _ := fn(nil, nil, "")
	if vals[0] != "x" {
		t.Errorf("staticCompletion did not defensive-copy; got %v", vals)
	}
}

func TestIntMapCompletion(t *testing.T) {
	fn := intMapCompletion(map[int]string{1: "見積中", 3: "受注済"})
	vals, dir := fn(nil, nil, "")
	want := []string{"1\t見積中", "3\t受注済"} // キー昇順
	if !reflect.DeepEqual(vals, want) {
		t.Errorf("intMapCompletion values = %v, want %v", vals, want)
	}
	if dir != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("intMapCompletion directive = %v, want NoFileComp", dir)
	}
}

func TestIntMapCompletion_SortedAscending(t *testing.T) {
	// キーが乱順でも昇順ソートされること
	fn := intMapCompletion(map[int]string{5: "受注済", 1: "見積中(高)", 3: "見積中(低)", 8: "見積中(除)"})
	vals, _ := fn(nil, nil, "")
	want := []string{"1\t見積中(高)", "3\t見積中(低)", "5\t受注済", "8\t見積中(除)"}
	if !reflect.DeepEqual(vals, want) {
		t.Errorf("intMapCompletion sort order wrong; got %v, want %v", vals, want)
	}
}

func TestIntMapCompletion_EmptyMap(t *testing.T) {
	fn := intMapCompletion(map[int]string{})
	vals, dir := fn(nil, nil, "")
	if len(vals) != 0 {
		t.Errorf("intMapCompletion(empty) = %v, want empty", vals)
	}
	if dir != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("intMapCompletion(empty) directive = %v, want NoFileComp", dir)
	}
}

// ------------------------------------------------------------
// CLI コマンドへの CompletionFunc 登録テスト
// ------------------------------------------------------------

// assertFlagCompletionRegistered: 指定 cmd の指定 flag に completion func が登録されていること
func assertFlagCompletionRegistered(t *testing.T, cmd *cobra.Command, flagName string) {
	t.Helper()
	fn, ok := cmd.GetFlagCompletionFunc(flagName)
	if !ok || fn == nil {
		t.Errorf("%s: flag %q has no completion func registered", cmd.Name(), flagName)
	}
}

// assertFlagCompletionNotRegistered: 指定 flag に completion func が登録されていないこと
func assertFlagCompletionNotRegistered(t *testing.T, cmd *cobra.Command, flagName string) {
	t.Helper()
	_, ok := cmd.GetFlagCompletionFunc(flagName)
	if ok {
		t.Errorf("%s: flag %q should NOT have completion func registered (value set未確認のため)", cmd.Name(), flagName)
	}
}

func findSubCmd(t *testing.T, parent *cobra.Command, name string) *cobra.Command {
	t.Helper()
	for _, c := range parent.Commands() {
		if c.Name() == name {
			return c
		}
	}
	t.Fatalf("subcommand %q not found under %q", name, parent.Name())
	return nil
}

func TestRegisterCompletions_APIClients(t *testing.T) {
	root := NewAPIClientsCmd()
	list := findSubCmd(t, root, "list")
	assertFlagCompletionRegistered(t, list, "response-group")
}

func TestRegisterCompletions_APIInvoices(t *testing.T) {
	root := NewAPIInvoicesCmd()
	list := findSubCmd(t, root, "list")
	assertFlagCompletionRegistered(t, list, "response-group")
	// status-eq は値未確認なので補完登録しないこと
	assertFlagCompletionNotRegistered(t, list, "status-eq")
}

func TestRegisterCompletions_APIPayments(t *testing.T) {
	root := NewAPIPaymentsCmd()
	list := findSubCmd(t, root, "list")
	assertFlagCompletionRegistered(t, list, "response-group")
	assertFlagCompletionNotRegistered(t, list, "status-eq")
}

func TestRegisterCompletions_APIPurchaseOrders(t *testing.T) {
	root := NewAPIPurchaseOrdersCmd()
	list := findSubCmd(t, root, "list")
	assertFlagCompletionRegistered(t, list, "response-group")
	assertFlagCompletionNotRegistered(t, list, "status-eq")
}

func TestRegisterCompletions_APIProjectsList(t *testing.T) {
	root := NewAPIProjectsCmd()
	list := findSubCmd(t, root, "list")
	assertFlagCompletionRegistered(t, list, "response-group")
	assertFlagCompletionRegistered(t, list, "order-status-in")
	assertFlagCompletionRegistered(t, list, "delivery-status-in")
	assertFlagCompletionRegistered(t, list, "invoice-timing-kbn-in")
}

func TestRegisterCompletions_APIProjectsGet(t *testing.T) {
	root := NewAPIProjectsCmd()
	get := findSubCmd(t, root, "get")
	assertFlagCompletionRegistered(t, get, "response-group")
}
