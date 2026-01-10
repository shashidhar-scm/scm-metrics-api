PACKAGE_NAME ?= scm-metrics-installer
VERSION_FILE ?= VERSION
ifeq ($(wildcard $(VERSION_FILE)),)
VERSION ?= $(shell V=$$(git describe --tags --always --dirty 2>/dev/null || echo dev); if echo $$V | grep -Eq '^[0-9]'; then echo $$V; else echo 0.0.0+dev; fi)
else
VERSION ?= $(shell cat $(VERSION_FILE))
endif
BUILD_DIR ?= build
PKG_NAME := $(PACKAGE_NAME)_$(VERSION)
PKG_DIR := $(BUILD_DIR)/$(PKG_NAME)
INSTALL_ROOT := $(PKG_DIR)/opt/scm-metrics
DEBIAN_ROOT := $(PKG_DIR)/DEBIAN
INSTALLER_DIR := installer
DEB_FILE := $(BUILD_DIR)/$(PKG_NAME).deb

.PHONY: installer-clean installer-deb installer-deb-docker bump-version

installer-clean:
	@echo "Cleaning installer artifacts"
	rm -rf $(PKG_DIR) $(DEB_FILE)

installer-deb: installer-clean
	@echo "Building $(DEB_FILE)"
	mkdir -p $(INSTALL_ROOT)
	cp -R $(INSTALLER_DIR)/bin $(INSTALL_ROOT)/
	cp -R $(INSTALLER_DIR)/configs $(INSTALL_ROOT)/
	cp -R $(INSTALLER_DIR)/lib $(INSTALL_ROOT)/ 2>/dev/null || true
	cp $(INSTALLER_DIR)/install.sh $(INSTALL_ROOT)/
	chmod 0755 $(INSTALL_ROOT)/install.sh
	chmod 0755 $(INSTALL_ROOT)/bin/*.sh
	mkdir -p $(DEBIAN_ROOT)
	sed "s/@VERSION@/$(VERSION)/" $(INSTALLER_DIR)/DEBIAN/control > $(DEBIAN_ROOT)/control
	cp $(INSTALLER_DIR)/DEBIAN/postinst $(DEBIAN_ROOT)/
	cp $(INSTALLER_DIR)/DEBIAN/prerm $(DEBIAN_ROOT)/
	chmod 0755 $(DEBIAN_ROOT)/postinst $(DEBIAN_ROOT)/prerm
	dpkg-deb --build $(PKG_DIR) $(DEB_FILE)
	@echo "Created $(DEB_FILE)"

installer-deb-docker:
	@echo "Building installer via Docker (Debian stable)"
	docker run --rm \
		-v $(PWD):/app \
		-w /app \
		-e VERSION=$(VERSION) \
		debian:stable-slim \
		bash -c "export DEBIAN_FRONTEND=noninteractive && apt-get update >/dev/null && apt-get install -y make dpkg-dev >/dev/null && make installer-deb"

bump-version:
	@echo "Bumping patch version..."
	@if [ ! -f $(VERSION_FILE) ]; then echo "0.0.1" > $(VERSION_FILE); else \
	  current=$$(cat $(VERSION_FILE)); \
	  major=$$(echo $$current | cut -d. -f1); \
	  minor=$$(echo $$current | cut -d. -f2); \
	  patch=$$(echo $$current | cut -d. -f3); \
	  if [ -z "$$patch" ]; then patch=0; fi; \
	  new_patch=$$((patch + 1)); \
	  printf "%s.%s.%s\n" $$major $$minor $$new_patch > $(VERSION_FILE); \
	fi
	@echo "New version: $$(cat $(VERSION_FILE))"
