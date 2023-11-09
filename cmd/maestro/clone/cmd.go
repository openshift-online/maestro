package clone

import (
	"flag"
	"fmt"
	"github.com/golang/glog"
	"github.com/openshift-online/maestro/pkg/config"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"os"
	"path/filepath"
	"strings"
)

type provisionCfgFlags struct {
	Name        string
	Destination string
}

func (c *provisionCfgFlags) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&c.Name, "name", c.Name, "Name of the new service being provisioned")
	fs.StringVar(&c.Destination, "destination", c.Destination, "Target directory for the newly provisioned instance")
}

var provisionCfg = &provisionCfgFlags{
	Name:        "maestro",
	Destination: "/tmp/clone-test",
}

// migrate sub-command handles running migrations
func NewCloneCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clone",
		Short: "Clone a new Maestro instance",
		Long:  "Clone a new Maestro instance",
		Run:   clone,
	}

	provisionCfg.AddFlags(cmd.PersistentFlags())
	cmd.PersistentFlags().AddGoFlagSet(flag.CommandLine)
	return cmd
}

var rw os.FileMode = 0777

func clone(_ *cobra.Command, _ []string) {

	glog.Infof("creating new Maestro instance as %s in directory %s", provisionCfg.Name, provisionCfg.Destination)

	// walk the filesystem, starting at the root of the project
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// ignore git subdirectories
		if path == ".git" || strings.Contains(path, ".git/") {
			return nil
		}

		dest := provisionCfg.Destination + "/" + path
		if strings.Contains(dest, "maestro") {
			dest = strings.Replace(dest, "maestro", strings.ToLower(provisionCfg.Name), -1)
		}

		if info.IsDir() {
			// does this path exist in the destination?
			if _, err := os.Stat(dest); os.IsNotExist(err) {
				glog.Infof("Directory does not exist, creating: %s", dest)
			}

			err := os.MkdirAll(dest, rw)
			if err != nil {
				return err
			}

		} else {
			content, err := config.ReadFile(path)
			if err != nil {
				return err
			}

			if strings.Contains(content, "Maestro") {
				glog.Infof("find/replace required for file: %s", path)
				content = strings.Replace(content, "Maestro", provisionCfg.Name, -1)
			}

			if strings.Contains(content, "maestro") {
				glog.Infof("find/replace required for file: %s", path)
				content = strings.Replace(content, "maestro", strings.ToLower(provisionCfg.Name), -1)
			}

			if strings.Contains(content, "maestro") {
				glog.Infof("find/replace required for file: %s", path)
				content = strings.Replace(content, "maestro", strings.ToLower(provisionCfg.Name), -1)
			}

			if strings.Contains(content, "maestro") {
				glog.Infof("find/replace required for file: %s", path)
				content = strings.Replace(content, "maestro", strings.ToLower(provisionCfg.Name), -1)
			}

			if strings.Contains(content, "Maestro") {
				glog.Infof("find/replace required for file: %s", path)
				content = strings.Replace(content, "Maestro", provisionCfg.Name, -1)
			}

			file, err := os.OpenFile(dest, os.O_APPEND|os.O_CREATE|os.O_RDWR, rw)
			if err != nil {
				return err
			}

			written, fErr := file.WriteString(content)
			if fErr != nil {
				return fErr
			}

			glog.Infof("wrote %d bytes for file %s", written, dest)
			file.Sync()
			file.Close()
		}

		return nil
	})

	if err != nil {
		fmt.Println(err)
	}

}
