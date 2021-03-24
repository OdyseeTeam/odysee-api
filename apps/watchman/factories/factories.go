package factories

import (
	"math/rand"

	"github.com/lbryio/lbrytv/apps/watchman/db"
)

func GeneratePlaybackReport() db.CreatePlaybackReportParams {
	return db.CreatePlaybackReportParams{
		URL: "lbry://" + randomString(12) + "#" + randomString(16),
		Pos: rand.Int31n(100_000_000),
		Por: rand.Int31n(10000),
		Dur: rand.Int31n(3_600_000),
		Bfc: rand.Int31n(10),
		Bfd: rand.Int31n(5000),
		Fmt: "std",
		Pid: randomStringItem([]string{"player1", "player2", "player3", "player4"}),
		Cid: randomString(32),
		Cdv: randomStringItem([]string{"ios", "and", "web"}),
		Crt: rand.Int31n(10_000_000),
		Car: randomStringItem([]string{"ce", "we", "ee", "use", "usw", "sea", "au"}),
	}
}

func randomString(n int) string {
	var letter = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

	b := make([]rune, n)
	for i := range b {
		b[i] = letter[rand.Intn(len(letter))]
	}
	return string(b)
}

func randomStringItem(in []string) string {
	return in[rand.Intn(len(in))]
}
