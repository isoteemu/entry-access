# Configurable directory for build artifacts
DIST_DIR ?= dist

# Font download URL base
FONT_BASE_URL ?= https://www.jyu.fi/themes/custom/jyu/fonts

# Detect available download tool
DOWNLOAD_CMD := $(shell command -v curl >/dev/null 2>&1 && echo "curl -o" || echo "wget -O")

# Create dist directory if it doesn't exist
$(DIST_DIR):
	mkdir -p $(DIST_DIR)
	mkdir -p $(DIST_DIR)/assets

# Download sas-emoji.json from matrix-spec repository
$(DIST_DIR)/assets/sas-emoji.json: $(DIST_DIR)
	@echo "Downloading sas-emoji spec"
	$(DOWNLOAD_CMD) $(DIST_DIR)/assets/sas-emoji.json https://raw.githubusercontent.com/matrix-org/matrix-spec/main/data-definitions/sas-emoji.json


# Download fonts
$(DIST_DIR)/assets/fonts: $(DIST_DIR)
	@echo "Downloading fonts"
	mkdir -p $(DIST_DIR)/assets/fonts
	$(DOWNLOAD_CMD) $(DIST_DIR)/assets/fonts/Aleo-Regular.otf $(FONT_BASE_URL)/aleo/Aleo-Regular.otf
	$(DOWNLOAD_CMD) $(DIST_DIR)/assets/fonts/Aleo-Bold.otf $(FONT_BASE_URL)/aleo/Aleo-Bold.otf
	$(DOWNLOAD_CMD) $(DIST_DIR)/assets/fonts/Lato-Regular.ttf $(FONT_BASE_URL)/Lato/Lato-Regular.ttf
	$(DOWNLOAD_CMD) $(DIST_DIR)/assets/fonts/Lato-Black.ttf $(FONT_BASE_URL)/Lato/Lato-Black.ttf
	$(DOWNLOAD_CMD) $(DIST_DIR)/assets/fonts/Lato-Bold.ttf $(FONT_BASE_URL)/Lato/Lato-Bold.ttf

# Compile CSS with Tailwind
$(DIST_DIR)/assets/css/output.css: web/assets/css/input.css $(DIST_DIR)
	@echo "Compiling CSS with Tailwind"
	mkdir -p $(DIST_DIR)/assets/css
	tailwindcss -i ./web/assets/css/input.css -o $(DIST_DIR)/assets/css/output.css

# Target to download emoji data
download-emoji-spec: $(DIST_DIR)/assets/sas-emoji.json

# Target to download all fonts
download-fonts: $(DIST_DIR)/assets/fonts

# Target to compile CSS
compile-css: $(DIST_DIR)/assets/css/output.css

# Target to download all assets
assets: download-emoji-spec download-fonts compile-css

.PHONY: download-emoji-spec download-fonts compile-css assets
