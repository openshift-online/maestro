# define the variables
REPO_URL = https://github.com/Azure/ARO-HCP.git
BRANCH = maestro-light-setup
CLONE_DIR = aro-hcp

# clone the repo
clone:
	@if [ -d $(CLONE_DIR) ]; then \
		echo "Removing existing directory $(CLONE_DIR)..."; \
		rm -rf $(CLONE_DIR); \
	fi; \
	echo "Cloning repository..."; \
	git clone $(REPO_URL) -b $(BRANCH) $(CLONE_DIR)
.PHONY: clone

# create the cluster (svc-cluster or mgmt-cluster)
cluster: clone
ifndef AKSCONFIG
	$(error "Must set AKSCONFIG")
endif
	@$(MAKE) -C $(CLONE_DIR)/dev-infrastructure cluster
.PHONY: cluster

# grant admin access to the cluster
aks.admin-access:
ifndef AKSCONFIG
	$(error "Must set AKSCONFIG")
endif
	@$(MAKE) -C $(CLONE_DIR)/dev-infrastructure aks.admin-access
.PHONY: aks.admin-access

# retrieve the kubeconfig
aks.kubeconfig:
ifndef AKSCONFIG
	$(error "Must set AKSCONFIG")
endif
	@$(MAKE) -C $(CLONE_DIR)/dev-infrastructure aks.kubeconfig
.PHONY: aks.kubeconfig

# deploy the maestro server
deploy-server:
	@AKSCONFIG=svc-cluster $(MAKE) -C $(CLONE_DIR)/maestro deploy-server
.PHONY: deploy-server

# deploy the maestro agent
deploy-agent:
	@AKSCONFIG=mgmt-cluster $(MAKE) -C $(CLONE_DIR)/maestro deploy-agent
.PHONY: deploy-agent

# register the maestro agent
register-agent:
	@AKSCONFIG=svc-cluster $(MAKE) -C $(CLONE_DIR)/maestro register-agent
.PHONY: register-agent

# enable the aks metrics
enable-aks-metrics: clone
ifndef AKSCONFIG
	$(error "Must set AKSCONFIG")
endif
	@$(MAKE) -C $(CLONE_DIR)/dev-infrastructure enable-aks-metrics
.PHONY: enable-aks-metrics

# clean up the resources
clean:
ifndef AKSCONFIG
	$(error "Must set AKSCONFIG")
endif
	@$(MAKE) -C $(CLONE_DIR)/dev-infrastructure clean
.PHONY: clean
