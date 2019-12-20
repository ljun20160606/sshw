.PHONY: install

build:
	@source ./build.sh && BuildAll

install:
	@echo 'go install'
	@cd cmd/sshw && go install

install/darwin:
	@source ./build.sh && InstallDarwin
