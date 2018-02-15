package keyring

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func init() {
	supportedBackends[PassBackend] = opener(func(cfg Config) (Keyring, error) {
		return &passKeyring{
			prefix: "aws-vault/",
		}, nil
	})
}

type passKeyring struct {
	dir     string
	passcmd string
	prefix  string
}

func (k *passKeyring) pass(args ...string) (*exec.Cmd, error) {
	cmd := exec.Command(k.passcmd, args...)
	if k.dir != "" {
		cmd.Env = append(os.Environ(), fmt.Sprintf("PASSWORD_STORE_DIR=%s", k.dir))
	}
	cmd.Stderr = os.Stderr

	return cmd, nil
}

func (k *passKeyring) Get(key string) (Item, error) {
	name := fmt.Sprintf("%s%s", k.prefix, key)
	cmd, err := k.pass("show", name)
	if err != nil {
		return Item{}, err
	}

	output, err := cmd.Output()
	if err != nil {
		return Item{}, err
	}

	var decoded Item
	err = json.Unmarshal(output, &decoded)

	return decoded, err
}

func (k *passKeyring) Set(i Item) error {
	bytes, err := json.Marshal(i)
	if err != nil {
		return err
	}

	name := fmt.Sprintf("%s%s", k.prefix, i.Key)
	cmd, err := k.pass("insert", "-m", "-f", name)
	if err != nil {
		return err
	}

	cmd.Stdin = strings.NewReader(string(bytes))

	err = cmd.Run()
	if err != nil {
		return err
	}

	return nil
}

func (k *passKeyring) Remove(key string) error {
	name := fmt.Sprintf("%s%s", k.prefix, key)
	_, err := k.pass("rm", name)
	if err != nil {
		return err
	}

	return nil
}

func (k *passKeyring) Keys() ([]string, error) {
	var keys = []string{}
	var path = ""

	// TODO This is admittedly lazy; it assumes k.prefix always _ends_ with a "/". If it doesn't, and is instead something like "a/b-" then you'll end up breaking ioutil.ReadDir later
	if strings.Contains(k.prefix, "/") {
		path = fmt.Sprintf("%s/%s", k.dir, k.prefix)
	} else {
		path = k.dir
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return keys, nil
		}
		return keys, err
	}
	if !info.IsDir() {
		return keys, errors.New(fmt.Sprintf("%s is not a directory", path))
	}

	files, err := ioutil.ReadDir(path)
	if err != nil {
		return keys, err
	}

	for _, f := range files {
		if filepath.Ext(f.Name()) == ".gpg" {
			name := filepath.Base(f.Name())
			keys = append(keys, name[:len(name)-4])

		}
	}

	return keys, nil
}