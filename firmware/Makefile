IDF_IMAGE ?= espressif/idf:v5.4.1
IDF_TARGET ?= esp32
DOCKER_IDF = docker run --rm -v $(CURDIR):/project -w /project -e HOME=/tmp $(IDF_IMAGE)
HOST_BUILD ?= $(CURDIR)/build-host

.PHONY: test build wokwi clean

test:
	cmake -S test/host -B $(HOST_BUILD) -DCMAKE_BUILD_TYPE=Debug
	cmake --build $(HOST_BUILD)
	ctest --test-dir $(HOST_BUILD) --output-on-failure

build:
	$(DOCKER_IDF) idf.py set-target $(IDF_TARGET) build

wokwi:
	$(DOCKER_IDF) idf.py -D "SDKCONFIG_DEFAULTS=sdkconfig.defaults;sdkconfig.defaults.wokwi" set-target $(IDF_TARGET) build

clean:
	rm -rf build build-host sdkconfig sdkconfig.old managed_components
