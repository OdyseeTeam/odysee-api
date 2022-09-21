package test

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"text/template"

	"github.com/ybbus/jsonrpc"
)

var SDKAddress = "http://localhost:15279"

type SDKWallet struct {
	UserID                int
	PrivateKey, PublicKey string
}

const (
	envPublicKey  = "REAL_WALLET_PUBLIC_KEY"
	envPrivateKey = "REAL_WALLET_PRIVATE_KEY"
)

func (w SDKWallet) walletID() string {
	return fmt.Sprintf("lbrytv-id.%d.wallet", w.UserID)
}

func (w SDKWallet) Inject() error {
	wt, err := template.New("wallet.template").Parse(tmplWallet)
	if err != nil {
		return err
	}
	wf, err := os.CreateTemp("", fmt.Sprintf("wallet.%v.*", w.UserID))
	if err != nil {
		return err
	}
	defer os.Remove(wf.Name())
	defer wf.Close()
	err = wt.Execute(wf, w)
	if err != nil {
		return err
	}

	err = w.Unload() // Unload an old wallet file before overwriting
	if err != nil {
		return err
	}

	err = copyToContainer(
		wf.Name(),
		fmt.Sprintf("lbrynet:/storage/lbryum/wallets/%s", w.walletID()),
	)
	if err != nil {
		return err
	}

	err = w.Load() // Unload an old wallet file before overwriting
	if err != nil {
		return err
	}

	return nil
}

func (w SDKWallet) Unload() error {
	c := jsonrpc.NewClient(SDKAddress)
	_, err := c.Call("wallet_remove", map[string]string{"wallet_id": w.walletID()})
	return err
}

func (w SDKWallet) Load() error {
	c := jsonrpc.NewClient(SDKAddress)
	_, err := c.Call("wallet_add", map[string]string{"wallet_id": w.walletID()})
	return err
}

func (w SDKWallet) RemoveFile() error {
	return rmFromContainer(fmt.Sprintf("/storage/lbryum/wallets/%s", w.walletID()))
}

func InjectTestingWallet(userID int) (*SDKWallet, error) {
	if os.Getenv(envPrivateKey) == "" || os.Getenv(envPublicKey) == "" {
		return nil, errors.New("missing env variables for test wallet")
	}
	w := SDKWallet{PrivateKey: os.Getenv(envPrivateKey), PublicKey: os.Getenv(envPublicKey), UserID: userID}

	err := w.Unload()
	if err != nil {
		return nil, fmt.Errorf("error unloading wallet: %w", err)
	}

	return &w, w.Inject()
}

func copyToContainer(srcPath, dstPath string) error {
	os.Setenv("PATH", os.Getenv("PATH")+":/usr/local/bin")
	if out, err := exec.Command("docker", "cp", srcPath, dstPath).CombinedOutput(); err != nil {
		return fmt.Errorf("cannot copy %s to %s: %w (%s)", srcPath, dstPath, err, string(out))
	}
	return nil
}

func rmFromContainer(p string) error {
	os.Setenv("PATH", os.Getenv("PATH")+":/usr/local/bin")
	if out, err := exec.Command("docker", "exec", "lbrynet", "rm", p).CombinedOutput(); err != nil {
		return fmt.Errorf("cannot remove lbrynet:%s: %w (%s)", p, err, string(out))
	}
	return nil
}
