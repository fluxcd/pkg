CRD := fake.toolkit.fluxcd.io_fakes.yaml

$(CRD):
	controller-gen rbac:roleName=manager-role crd paths=./../ output:crd:dir=./ output:stdout
.PHONY: $(CRD)
