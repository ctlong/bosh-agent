// +build !windows

package bundlecollection_test

import (
	"errors"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/agent/applier/bundlecollection"
	"github.com/cloudfoundry/bosh-agent/agent/applier/bundlecollection/fakes"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
)

//go:generate counterfeiter -o fakes/fake_clock.go ../../../vendor/code.cloudfoundry.org/clock Clock

var _ = Describe("FileBundle", func() {
	var (
		fs          *fakesys.FakeFileSystem
		fakeClock   *fakes.FakeClock
		logger      boshlog.Logger
		sourcePath  string
		installPath string
		enablePath  string
		fileBundle  FileBundle
	)

	BeforeEach(func() {
		fs = fakesys.NewFakeFileSystem()
		fakeClock = new(fakes.FakeClock)
		installPath = "/install-path"
		enablePath = "/enable-path"
		logger = boshlog.NewLogger(boshlog.LevelNone)
		fileBundle = NewFileBundle(installPath, enablePath, os.FileMode(0750), fs, fakeClock, logger)
	})

	createSourcePath := func() string {
		path := "/source-path"
		err := fs.MkdirAll(path, os.ModePerm)
		Expect(err).ToNot(HaveOccurred())

		err = fs.WriteFileString("/source-path/config.go", "package go")
		Expect(err).ToNot(HaveOccurred())

		return path
	}

	BeforeEach(func() {
		sourcePath = createSourcePath()
	})

	Describe("Install", func() {
		It("handles rename failing by falling back on copying the directory (cross-device link errors)", func() {
			fs.RenameError = errors.New("fake-rename-error")

			_, err := fileBundle.Install(sourcePath)
			Expect(err).NotTo(HaveOccurred())

			installed, err := fileBundle.IsInstalled()
			Expect(err).NotTo(HaveOccurred())
			Expect(installed).To(BeTrue(), "Bundle not installed")

			Expect(fs.FileExists(sourcePath)).To(BeFalse())
		})

		It("returns error when moving source to install path fails", func() {
			fs.RenameError = errors.New("fake-rename-error")
			fs.CopyDirError = errors.New("fake-copy-dir-error")

			_, err := fileBundle.Install(sourcePath)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-copy-dir-error"))
		})

		It("installs the bundle from source at the given path", func() {
			path, err := fileBundle.Install(sourcePath)
			Expect(err).NotTo(HaveOccurred())
			Expect(path).To(Equal(installPath))

			installed, err := fileBundle.IsInstalled()
			Expect(err).NotTo(HaveOccurred())
			Expect(installed).To(BeTrue(), "Bundle not installed")

			contents, err := fs.ReadFileString(filepath.Join(path, "config.go"))
			Expect(err).NotTo(HaveOccurred())
			Expect(contents).To(Equal("package go"))
		})

		It("returns an error if creation of parent directory fails", func() {
			fs.MkdirAllError = errors.New("fake-mkdir-error")

			_, err := fileBundle.Install(sourcePath)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-mkdir-error"))
		})

		It("sets correct permissions on install path", func() {
			fs.Chmod(sourcePath, os.FileMode(0700))

			_, err := fileBundle.Install(sourcePath)
			Expect(err).NotTo(HaveOccurred())

			fileStats := fs.GetFileTestStat(installPath)
			Expect(fileStats).ToNot(BeNil())
			Expect(fileStats.FileType).To(Equal(fakesys.FakeFileType(fakesys.FakeFileTypeDir)))
			Expect(fileStats.FileMode).To(Equal(os.FileMode(0750)))
			Expect(fileStats.Username).To(Equal("root"))
			Expect(fileStats.Groupname).To(Equal("vcap"))
		})

		It("is idempotent", func() {
			path, err := fileBundle.Install(sourcePath)
			Expect(err).NotTo(HaveOccurred())
			Expect(path).To(Equal(installPath))

			sourcePath = createSourcePath()
			path, err = fileBundle.Install(sourcePath)
			Expect(err).NotTo(HaveOccurred())
			Expect(path).To(Equal(installPath))
		})

		It("returns error when it fails to change permissions", func() {
			fs.ChmodErr = errors.New("fake-chmod-error")

			_, err := fileBundle.Install(sourcePath)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake-chmod-error"))
		})

		It("does not install bundle if it fails to change permissions", func() {
			fs.ChmodErr = errors.New("fake-chmod-error")

			_, err := fileBundle.Install(sourcePath)
			Expect(err).To(HaveOccurred())
			Expect(fs.FileExists(installPath)).To(BeFalse())
		})
	})
})
