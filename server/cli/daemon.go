package cli

import (
	"fmt"
	"log"
	"os"
	"runtime/debug"

	"github.com/bishopfox/sliver/client/constants"
	"github.com/bishopfox/sliver/server/assets"
	"github.com/bishopfox/sliver/server/c2"
	"github.com/bishopfox/sliver/server/certs"
	"github.com/bishopfox/sliver/server/console"
	"github.com/bishopfox/sliver/server/cryptography"
	"github.com/bishopfox/sliver/server/daemon"
	"github.com/bishopfox/sliver/server/db"
	"github.com/bishopfox/sliver/server/db/models"
	"github.com/spf13/cobra"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Force start server in daemon mode",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		force, err := cmd.Flags().GetBool(forceFlagStr)
		if err != nil {
			fmt.Printf("Failed to parse --%s flag %s\n", forceFlagStr, err)
			return
		}
		lhost, err := cmd.Flags().GetString(lhostFlagStr)
		if err != nil {
			fmt.Printf("Failed to parse --%s flag %s\n", lhostFlagStr, err)
			return
		}
		lport, err := cmd.Flags().GetUint16(lportFlagStr)
		if err != nil {
			fmt.Printf("Failed to parse --%s flag %s\n", lportFlagStr, err)
			return
		}

		appDir := assets.GetRootAppDir()
		logFile := initConsoleLogging(appDir)
		defer logFile.Close()

		defer func() {
			if r := recover(); r != nil {
				log.Printf("panic:\n%s", debug.Stack())
				fmt.Println("stacktrace from panic: \n" + string(debug.Stack()))
				os.Exit(99)
			}
		}()

		assets.Setup(force, false)
		certs.SetupCAs()
		certs.SetupWGKeys()
		cryptography.AgeServerKeyPair()
		cryptography.MinisignServerPrivateKey()

		listenerJobs, err := db.ListenerJobs()
		if err != nil {
			fmt.Println(err)
		}

		err = StartPersistentJobs(listenerJobs)
		if err != nil {
			fmt.Println(err)
		}

		daemon.Start(lhost, uint16(lport))
	},
}

func StartPersistentJobs(listenerJobs *[]models.ListenerJob) error {
	if len(*listenerJobs) > 0 {
		// StartPersistentJobs - Start persistent jobs
		for _, j := range *listenerJobs {
			listenerJob, err := db.ListenerByJobID(j.JobID)
			if err != nil {
				return err
			}
			switch j.Type {
			case constants.HttpStr:
				job, err := c2.StartHTTPListenerJob(listenerJob.ToProtobuf().HTTPConf)
				if err != nil {
					return err
				}
				j.JobID = uint32(job.ID)
			case constants.HttpsStr:
				job, err := c2.StartHTTPListenerJob(listenerJob.ToProtobuf().HTTPConf)
				if err != nil {
					return err
				}
				j.JobID = uint32(job.ID)
			case constants.MtlsStr:
				job, err := c2.StartMTLSListenerJob(listenerJob.MtlsListener.ToProtobuf())
				if err != nil {
					return err
				}
				j.JobID = uint32(job.ID)
			case constants.WGStr:
				job, err := c2.StartWGListenerJob(listenerJob.WgListener.ToProtobuf())
				if err != nil {
					return err
				}
				j.JobID = uint32(job.ID)
			case constants.DnsStr:
				job, err := c2.StartDNSListenerJob(listenerJob.DnsListener.ToProtobuf())
				if err != nil {
					return err
				}
				j.JobID = uint32(job.ID)
			case constants.MultiplayerModeStr:
				id, err := console.JobStartClientListener(listenerJob.MultiplayerListener.ToProtobuf())
				if err != nil {
					return err
				}
				j.JobID = uint32(id)
			}
			db.HTTPC2ListenerUpdate(&j)
		}
	}

	return nil
}
