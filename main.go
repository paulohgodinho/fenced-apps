package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/docker/go-connections/nat"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

type Image struct {
	name     string
	pullPath string
}

func main() {

	readFile, err := os.Open("available_images")
	if err != nil {
		fmt.Println(err)
	}
	fileScanner := bufio.NewScanner(readFile)
	fileScanner.Split(bufio.ScanLines)

	var images []Image
	for fileScanner.Scan() {
		fields := strings.Fields(fileScanner.Text())
		images = append(images, Image{fields[0], fields[1]})
	}

	for i, image := range images {
		fmt.Println(i, image.name)
	}

	readFile.Close()

	var selectedImage int
	fmt.Scan(&selectedImage)
	fmt.Println("Pulling image: ", images[selectedImage].name, " from ", images[selectedImage].pullPath)

	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}
	defer cli.Close()

	pullOutput, err := cli.ImagePull(ctx, images[selectedImage].pullPath, types.ImagePullOptions{})
	if err != nil {
		panic(err)
	}
	defer pullOutput.Close()

	reader := bufio.NewReader(pullOutput)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			panic(fmt.Sprintf("Error reading bytes: %v", err))
		}
		os.Stdout.Write(line)
	}

	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image: images[selectedImage].pullPath,
		Tty:   false,
		Env:   []string{"VNC_PW=password"},
	}, &container.HostConfig{
		PortBindings: nat.PortMap{"6901/tcp": []nat.PortBinding{{HostIP: "127.0.0.1", HostPort: "6901"}}},
	}, nil, nil, "")
	if err != nil {
		panic(err)
	}

	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		panic(err)
	}

	open("https://localhost:6901")

	statusCh, errCh := cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			panic(err)
		}
	case <-statusCh:
	}

	out, err := cli.ContainerLogs(ctx, resp.ID, types.ContainerLogsOptions{ShowStdout: true})
	if err != nil {
		panic(err)
	}

	stdcopy.StdCopy(os.Stdout, os.Stderr, out)
}

func open(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start"}
	case "darwin":
		cmd = "open"
	default: // "linux", "freebsd", "openbsd", "netbsd"
		cmd = "xdg-open"
	}
	args = append(args, url)
	return exec.Command(cmd, args...).Start()
}
