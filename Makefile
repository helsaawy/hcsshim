BASE:=base.tar.gz
DEV_BUILD:=0

GO:=go
GO_FLAGS:=-ldflags "-s -w" # strip Go binaries
CGO_ENABLED:=0
GOMODVENDOR:=

CFLAGS:=-O2 -Wall
LDFLAGS:=-static -s # strip C binaries

GO_FLAGS_EXTRA:=
ifeq "$(GOMODVENDOR)" "1"
GO_FLAGS_EXTRA += -mod=vendor
endif
GO_BUILD_TAGS:=
ifneq ($(strip $(GO_BUILD_TAGS)),)
GO_FLAGS_EXTRA += -tags="$(GO_BUILD_TAGS)"
endif
GO_BUILD:=CGO_ENABLED=$(CGO_ENABLED) $(GO) build $(GO_FLAGS) $(GO_FLAGS_EXTRA)

SRCROOT=$(dir $(abspath $(firstword $(MAKEFILE_LIST))))
# additional directories to search for rule prerequisites and targets
VPATH=$(SRCROOT)

DELTA_TARGET=out/delta.win.tar.gz

ifeq "$(DEV_BUILD)" "1"
DELTA_TARGET=out/delta-dev.tar.gz
endif

# The link aliases for gcstools
GCS_TOOLS=\
	generichook \
	install-drivers

.PHONY: all always rootfs test

.DEFAULT_GOAL := all

all: out/initrd.img out/rootfs.tar.gz

clean:
	find -name '*.o' -print0 | xargs -0 -r rm
	rm -rf bin deps out/rootfs out

test:
	cd $(SRCROOT) && $(GO) test -v ./internal/guest/...

rootfs: out/rootfs.vhd

out/rootfs.vhd: out/rootfs.tar.gz bin/cmd/tar2ext4
	gzip -f -d ./out/rootfs.tar.gz
	bin/cmd/tar2ext4 -vhd -i ./out/rootfs.tar -o $@

out/rootfs.tar.gz: out/initrd.img
	rm -rf out/rootfs-conv
	mkdir out/rootfs-conv
	gunzip -c out/initrd.img | (cd out/rootfs-conv && cpio -imd)
	tar -zcf $@ -C out/rootfs-conv .
	rm -rf out/rootfs-conv

out/initrd.img: $(BASE) $(DELTA_TARGET) $(SRCROOT)/hack/catcpio.sh
	$(SRCROOT)/hack/catcpio.sh "$(BASE)" $(DELTA_TARGET) > out/initrd.img.uncompressed
	gzip -c out/initrd.img.uncompressed > $@
	rm out/initrd.img.uncompressed

# This target includes utilities which may be useful for testing purposes.
out/delta-dev.tar.gz: out/delta.tar.gz bin/internal/tools/snp-report
	rm -rf out/rootfs-dev
	mkdir out/rootfs-dev
	tar -xzf out/delta.tar.gz -C out/rootfs-dev
	cp bin/internal/tools/snp-report out/rootfs-dev/bin/
	tar -zcf $@ -C out/rootfs-dev .
	rm -rf out/rootfs-dev

out/delta.tar.gz: bin/init bin/vsockexec bin/cmd/gcs bin/cmd/gcstools bin/cmd/hooks/wait-paths Makefile
	@mkdir -p out
	rm -rf out/rootfs
	mkdir -p out/rootfs/bin/
	mkdir -p out/rootfs/info/
	cp bin/init out/rootfs/
	cp bin/vsockexec out/rootfs/bin/
	cp bin/cmd/gcs out/rootfs/bin/
	cp bin/cmd/gcstools out/rootfs/bin/
	cp bin/cmd/hooks/wait-paths out/rootfs/bin/
	for tool in $(GCS_TOOLS); do ln -s gcstools out/rootfs/bin/$$tool; done
	git -C $(SRCROOT) rev-parse HEAD > out/rootfs/info/gcs.commit && \
	git -C $(SRCROOT) rev-parse --abbrev-ref HEAD > out/rootfs/info/gcs.branch && \
	date --iso-8601=minute --utc > out/rootfs/info/tar.date
	$(if $(and $(realpath $(subst .tar,.testdata.json,$(BASE))), $(shell which jq)), \
		jq -r '.IMAGE_NAME' $(subst .tar,.testdata.json,$(BASE)) 2>/dev/null > out/rootfs/info/image.name && \
		jq -r '.DATETIME' $(subst .tar,.testdata.json,$(BASE)) 2>/dev/null > out/rootfs/info/build.date)
	tar -zcf $@ -C out/rootfs .

-include deps/cmd/gcs.gomake
-include deps/cmd/gcstools.gomake
-include deps/cmd/hooks/wait-paths.gomake
-include deps/cmd/tar2ext4.gomake
-include deps/internal/tools/snp-report.gomake

# Implicit rule for includes that define Go targets.
%.gomake: $(SRCROOT)/Makefile
	@mkdir -p $(dir $@)
	@/bin/echo $(@:deps/%.gomake=bin/%): $(SRCROOT)/hack/gomakedeps.sh > $@.new
	@/bin/echo -e '\t@mkdir -p $$(dir $$@) $(dir $@)' >> $@.new
	@/bin/echo -e '\t$$(GO_BUILD) -o $$@.new $$(SRCROOT)/$$(@:bin/%=%)' >> $@.new
	@/bin/echo -e '\tGO="$(GO)" $$(SRCROOT)/hack/gomakedeps.sh $$@ $$(SRCROOT)/$$(@:bin/%=%) $$(GO_FLAGS) $$(GO_FLAGS_EXTRA) > $(@:%.gomake=%.godeps).new' >> $@.new
	@/bin/echo -e '\tmv $(@:%.gomake=%.godeps).new $(@:%.gomake=%.godeps)' >> $@.new
	@/bin/echo -e '\tmv $$@.new $$@' >> $@.new
	@/bin/echo -e '-include $(@:%.gomake=%.godeps)' >> $@.new
	mv $@.new $@

bin/vsockexec: vsockexec/vsockexec.o vsockexec/vsock.o
	@mkdir -p bin
	$(CC) $(LDFLAGS) -o $@ $^

bin/init: init/init.o vsockexec/vsock.o
	@mkdir -p bin
	$(CC) $(LDFLAGS) -o $@ $^

%.o: %.c
	@mkdir -p $(dir $@)
	$(CC) $(CFLAGS) $(CPPFLAGS) -c -o $@ $<
