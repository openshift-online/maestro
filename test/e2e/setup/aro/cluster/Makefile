SHELL = /bin/bash

include configurations/dev.mk

# CURRENTUSER = $(shell az ad signed-in-user show | jq -r '.id')
CURRENTUSER = $(shell az account show -o json | jq -r '.name')
DEPLOYMENTNAME = $(RESOURCEGROUP)
DEPLOYMENTNAMEREGION = $(REGIONAL_RESOURCEGROUP)-REGION

# Set SKIP_CONFIRM to a non-empty value to skip "what-if" confirmation prompts.
ifndef SKIP_CONFIRM
PROMPT_TO_CONFIRM = "--confirm-with-what-if"
else
PROMPT_TO_CONFIRM_CLEANUP = "--yes"
endif

list:
	@grep '^[^#[:space:]].*:' Makefile
.PHONY: list

fmt:
	set -e; \
	FILES="$$(find . -type f -name "*.bicep*")"; \
	for file in $$FILES; do \
	echo "az bicep format --file $${file}"; \
	az bicep format --file $$file; \
	done
.PHONY: fmt

lint:
	set -e; \
	FILES="$$(find . -type f -name "*.bicep*")"; \
	for file in $$FILES; do \
	echo "az bicep lint --file $${file}"; \
	az bicep lint --file $$file; \
	done
.PHONY: lint

feature-registration: # hardcoded to eastus as this is a subscription deployment, not a resource group
	@az deployment sub create \
		--name "feature-registration" \
		--location eastus \
		--template-file templates/feature-registration.bicep \
		$(PROMPT_TO_CONFIRM)
.PHONY: feature-registration

rg:
	@if [ "$$(az group exists --name $(RESOURCEGROUP))" = "false" ]; then \
		az group create \
			--name $(RESOURCEGROUP)  \
			--location $(REGION) \
			--output none; \
	fi
.PHONY: rg

regionalRg:
	@if [ "$$(az group exists --name $(REGIONAL_RESOURCEGROUP))" = "false" ]; then \
		az group create \
			--name $(REGIONAL_RESOURCEGROUP)  \
			--location $(REGION) \
			--output none; \
	fi
.PHONY: regionalRg

cleanup-orphaned-rolebindings:
	@scripts/cleanup-orphaned-rolebindings.sh $(RESOURCEGROUP)
	@scripts/cleanup-orphaned-rolebindings.sh $(REGIONAL_RESOURCEGROUP)
.PHONY: cleanup-orphaned-rolebindings

region: regionalRg
	az deployment group create \
		--name "$(DEPLOYMENTNAMEREGION)" \
		--resource-group $(REGIONAL_RESOURCEGROUP) \
		--template-file templates/region.bicep \
		$(PROMPT_TO_CONFIRM) \
		--parameters \
			configurations/region.bicepparam \
		--parameters \
			currentUserId=$(CURRENTUSER)
.PHONY: region

# cluster: rg cleanup-orphaned-rolebindings region acr
cluster: rg cleanup-orphaned-rolebindings region
ifndef AKSCONFIG
	$(error "Must set AKSCONFIG")
endif
	az deployment group create \
		--name "$(DEPLOYMENTNAME)" \
		--resource-group $(RESOURCEGROUP) \
		--template-file templates/$(AKSCONFIG).bicep \
		$(PROMPT_TO_CONFIRM) \
		--parameters \
			configurations/$(AKSCONFIG).bicepparam \
		--parameters \
			currentUserId=$(CURRENTUSER) \
			regionalResourceGroup=$(REGIONAL_RESOURCEGROUP)
.PHONY: cluster

cluster-what-if: rg
ifndef AKSCONFIG
	$(error "Must set AKSCONFIG")
endif
	az deployment group what-if \
		--name "$(DEPLOYMENTNAME)" \
		--resource-group $(RESOURCEGROUP) \
		--template-file templates/$(AKSCONFIG).bicep \
		--parameters \
			configurations/$(AKSCONFIG).bicepparam \
		--parameters \
			currentUserId=$(CURRENTUSER) \
.PHONY: cluster-what-if

aks.admin-access: rg
ifndef AKSCONFIG
	$(error "Must set AKSCONFIG")
endif
	@scripts/aks-admin-access.sh $(RESOURCEGROUP)
.PHONY: aks.admin-access

aks.kubeconfig:
ifndef AKSCONFIG
	$(error "Must set AKSCONFIG")
endif
	AKS_NAME="$$(az aks list --query "[?tags.clusterType == '$(AKSCONFIG)'].name" -ojson -g $(RESOURCEGROUP) | jq -r '.[0]')"; \
	az aks get-credentials -n "$${AKS_NAME}" -g $(RESOURCEGROUP) -f "${HOME}/.kube/$(AKSCONFIG).kubeconfig"
	kubelogin convert-kubeconfig -l azurecli --kubeconfig "${HOME}/.kube/$(AKSCONFIG).kubeconfig"
.PHONY: aks.kubeconfig

clean:
	az group delete -g $(RESOURCEGROUP) $(PROMPT_TO_CONFIRM_CLEANUP)
	KEY_VAULT_NAME=$(shell az keyvault list-deleted --query "[?properties.tags.resourceGroup == '$(RESOURCEGROUP)'].name" -ojson | jq -r '.[0]')
	if [ -n "$${KEY_VAULT_NAME}" ]; then \
		az keyvault purge --name "$${KEY_VAULT_NAME}"; \
	fi
.PHONY: clean
