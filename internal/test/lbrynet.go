package test

import (
	"fmt"
	"os"
	"os/exec"
	"text/template"
)

type SDKWallet struct {
	UserID                int
	PrivateKey, PublicKey string
}

const (
	envPublicKey  = "REAL_WALLET_PUBLIC_KEY"
	envPrivateKey = "REAL_WALLET_PRIVATE_KEY"
)

func copyToContainer(srcPath, dstPath string) error {
	// cmd := fmt.Sprintf(`docker cp %s %s`, srcPath, dstPath)
	os.Setenv("PATH", os.Getenv("PATH")+":/usr/local/bin")
	if out, err := exec.Command("docker", "cp", srcPath, dstPath).CombinedOutput(); err != nil {
		// if _, err := exec.Command("docker", "cp", srcPath, dstPath).Output(); err != nil {
		return fmt.Errorf("cannot copy %s to %s: %w (%s)", srcPath, dstPath, err, string(out))
	}
	return nil
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

	return copyToContainer(
		wf.Name(),
		fmt.Sprintf("lbrynet:/storage/lbryum/wallets/lbrytv-id.%v.wallet", w.UserID),
	)
}

func InjectTestingWallet(userID int) (SDKWallet, error) {
	w := SDKWallet{PrivateKey: os.Getenv(envPrivateKey), PublicKey: os.Getenv(envPublicKey), UserID: userID}
	return w, w.Inject()
}
