package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

const (
	crdbVolumeName  = "crdb_volume"
	crdbVolumeMount = "/cockroach-data"
)

var createVolumeCmd = &cobra.Command{
	Use:   "create-volume",
	Short: "Creates the CockroachDB data volume",
	Run: func(cmd *cobra.Command, args []string) {
		dir, _ := cmd.Flags().GetString("dir")
		force, _ := cmd.Flags().GetBool("force")

		if _, err := os.Stat(dir); os.IsNotExist(err) {
			fmt.Printf("Directory specified by --dir flag does not exist: %s. Exiting.\n", dir)
			os.Exit(1)
		}

		volumeExistsCmd := exec.Command("docker", "volume", "ls", "-f", "name="+crdbVolumeName, "-q")
		existingVolume, err := volumeExistsCmd.Output()
		if err != nil {
			fmt.Println("Error checking for docker volume:", err)
			os.Exit(1)
		}
		volumeExists := strings.TrimSpace(string(existingVolume)) != ""

		if force && volumeExists {
			fmt.Println("WARNING: This will permanently delete the existing CockroachDB data volume.")
			confirmation := promptForInput("Type DELETE to confirm:")
			if confirmation != "DELETE" {
				fmt.Println("Confirmation failed. Aborting.")
				return
			}

			fmt.Println("Destroying existing CockroachDB data volume:", crdbVolumeName)
			downCmd := exec.Command("docker", "compose", "down", "--volumes")
			downCmd.Stdout = os.Stdout
			downCmd.Stderr = os.Stderr
			if err := downCmd.Run(); err != nil {
				fmt.Println("Error running 'docker compose down --volumes':", err)
				os.Exit(1)
			}

			rmVolCmd := exec.Command("docker", "volume", "rm", "-f", crdbVolumeName)
			rmVolCmd.Stdout = os.Stdout
			rmVolCmd.Stderr = os.Stderr
			if err := rmVolCmd.Run(); err != nil {
				fmt.Println("Error removing docker volume:", err)
				os.Exit(1)
			}
			volumeExists = false
		}

		if !volumeExists {
			fmt.Println("Creating CockroachDB data volume...")
			createVolCmd := exec.Command("docker", "volume", "create", "--name", crdbVolumeName)
			if err := createVolCmd.Run(); err != nil {
				fmt.Println("Error creating docker volume:", err)
				os.Exit(1)
			}

			fmt.Println("Copying data to volume...")
			copyToVolume(dir)
		} else {
			fmt.Println("Volume", crdbVolumeName, "already exists. Use --force to recreate.")
		}

		fmt.Println("Volume setup complete.")
	},
}

func copyToVolume(srcPath string) {
	dockerBuildCmd := exec.Command("docker", "build", "-t", "empty", "-")
	dockerBuildCmd.Stdin = strings.NewReader("FROM scratch\nLABEL empty=''''")
	if err := dockerBuildCmd.Run(); err != nil {
		fmt.Println("Error creating empty docker image:", err)
		os.Exit(1)
	}

	containerCreateCmd := exec.Command("docker", "container", "create", "-v", crdbVolumeName+":"+crdbVolumeMount, "empty", "cmd")
	containerIDBytes, err := containerCreateCmd.Output()
	if err != nil {
		fmt.Println("Error creating container with volume:", err)
		os.Exit(1)
	}
	containerID := strings.TrimSpace(string(containerIDBytes))

	cpSrcPath := srcPath
	if !strings.HasSuffix(cpSrcPath, "/") {
		cpSrcPath += "/"
	}
	dockerCpCmd := exec.Command("docker", "cp", cpSrcPath, containerID+":"+crdbVolumeMount)
	dockerCpCmd.Stdout = os.Stdout
	dockerCpCmd.Stderr = os.Stderr
	if err := dockerCpCmd.Run(); err != nil {
		fmt.Println("Error copying data to volume:", err)
		exec.Command("docker", "rm", containerID, "--volumes").Run()
		os.Exit(1)
	}

	dockerRmCmd := exec.Command("docker", "rm", containerID, "--volumes")
	if err := dockerRmCmd.Run(); err != nil {
		fmt.Println("Error removing temporary container:", err)
		os.Exit(1)
	}
}

func init() {
	createVolumeCmd.GroupID = "setup"
	rootCmd.AddCommand(createVolumeCmd)
	createVolumeCmd.Flags().String("dir", "cockroach-data", "CockroachDB data directory to populate volume from")
	createVolumeCmd.Flags().Bool("force", false, "Force creation of volume even if it already exists (destroys existing volume)")
}
