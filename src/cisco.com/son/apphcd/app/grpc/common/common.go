// Author <dorzheho@cisco.com>

package common

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/viper"
	"golang.org/x/crypto/ssh"
	"io/ioutil"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	appcommon "cisco.com/son/apphcd/app/common"
	"cisco.com/son/apphcd/app/grpc/common/syncer"
)

const (
	gitRepoUserName  = "intucell"
	gitRepoPassword  = "intucell"
	gitRepoName      = "secrets"
	gitRepoTargetDir = "/tmp/.cache/secrets"
	gitRepoBranch    = "master"
)

func KubeClientset() (*kubernetes.Clientset, error) {
	config, err := clientcmd.BuildConfigFromFlags("", appcommon.ApphcKubeconfigPath)
	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(config)
}

func CreateSshClient() (*ssh.Client, error) {
	if err := syncer.Clone("http", gitRepoUserName, gitRepoPassword, viper.GetString(appcommon.EnvApphcGitServerEndpoint),
		gitRepoName, gitRepoTargetDir, gitRepoBranch); err != nil {
		return nil, err
	}

	defer os.RemoveAll(gitRepoTargetDir)

	key, err := ioutil.ReadFile(fmt.Sprintf("%s/%s/.ssh/id_rsa",gitRepoTargetDir, viper.GetString(appcommon.EnvApphMasterNodeUser)))
	if err != nil {
		return nil, err
	}

	signer, err := ssh.ParsePrivateKey(key)

	// SSH client config
	config := &ssh.ClientConfig{
		User: viper.GetString(appcommon.EnvApphMasterNodeUser),
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	// Connect to host
	client, err := ssh.Dial("tcp", viper.GetString(appcommon.EnvApphMasterNodeIp)+":22", config)
	if err != nil {
		return nil, err
	}

	return client, nil
}


func GetStorageGbInt(storageSize string) (int, error) {
	var storage string
	inGb := true

	if strings.HasSuffix(storageSize, "Ki") {
		storage = storageSize[0:(len(storageSize) - 2)]
		inGb = false
	} else if strings.HasSuffix(storageSize, "Gi")  {
		storage = storageSize[0:(len(storageSize) - 2)]
	} else {
		storage = storageSize
	}

	storageInt, err := strconv.Atoi(storage)
	if err != nil {
		return 0, err
	}

	if inGb {
		return storageInt, nil
	}

	return storageInt / 1024, nil
}
