package api_test

import (
	svcapi "github.com/youyo/board/internal/service/api"
)

// newServiceWithClients は clients スタブのみを差し込んだ Service を返す。
func newServiceWithClients(stub *stubClientRepo) *svcapi.Service {
	return svcapi.New(svcapi.Repos{
		Clients:        stub,
		ClientBranches: &stubClientBranchRepo{},
		Contacts:       &stubContactRepo{},
		Projects:       &stubProjectRepo{},
		ProjectCosts:   &stubProjectCostRepo{},
	})
}

// newServiceWithClientBranches は clientBranches スタブのみを差し込んだ Service を返す。
func newServiceWithClientBranches(stub *stubClientBranchRepo) *svcapi.Service {
	return svcapi.New(svcapi.Repos{
		Clients:        &stubClientRepo{},
		ClientBranches: stub,
		Contacts:       &stubContactRepo{},
		Projects:       &stubProjectRepo{},
		ProjectCosts:   &stubProjectCostRepo{},
	})
}

// newServiceWithContacts は contacts スタブのみを差し込んだ Service を返す。
func newServiceWithContacts(stub *stubContactRepo) *svcapi.Service {
	return svcapi.New(svcapi.Repos{
		Clients:        &stubClientRepo{},
		ClientBranches: &stubClientBranchRepo{},
		Contacts:       stub,
		Projects:       &stubProjectRepo{},
		ProjectCosts:   &stubProjectCostRepo{},
	})
}

// newServiceWithProjects は projects スタブのみを差し込んだ Service を返す。
func newServiceWithProjects(stub *stubProjectRepo) *svcapi.Service {
	return svcapi.New(svcapi.Repos{
		Clients:        &stubClientRepo{},
		ClientBranches: &stubClientBranchRepo{},
		Contacts:       &stubContactRepo{},
		Projects:       stub,
		ProjectCosts:   &stubProjectCostRepo{},
	})
}

// newServiceWithProjectCosts は projectCosts スタブのみを差し込んだ Service を返す。
func newServiceWithProjectCosts(stub *stubProjectCostRepo) *svcapi.Service {
	return svcapi.New(svcapi.Repos{
		Clients:        &stubClientRepo{},
		ClientBranches: &stubClientBranchRepo{},
		Contacts:       &stubContactRepo{},
		Projects:       &stubProjectRepo{},
		ProjectCosts:   stub,
	})
}
