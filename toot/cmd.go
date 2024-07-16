package toot

import (
	"fmt"
	"os"

	"github.com/mattn/go-mastodon"
)

func (s *Config) RegisterMotion(cfg MotionEye) {
	s.motion = cfg
}

func (s *Config) cmdMotionLast(status *mastodon.Status) error {
	ipath, err := s.motion.LastPhoto("1")
	if err != nil {
		return err
	}

	motImage, err := os.Open(ipath)
	if err != nil {
		return err
	}
	defer motImage.Close()

	var m []mastodon.ID
	if a, err := s.Client().UploadMediaFromReader(s.Ctx(), motImage); err != nil {
		return err
	} else {
		a.Description = "MotionEye Bild"
		m = []mastodon.ID{a.ID}
	}

	toot := mastodon.Toot{
		Status:      fmt.Sprintf("@%s\n\nHier ist dein gewünschtes Bild.", status.Account.Acct),
		InReplyToID: status.ID,
		MediaIDs:    m,
		Visibility:  mastodon.VisibilityDirectMessage,
		Language:    "de",
	}
	if _, err := s.Client().PostStatus(s.Ctx(), &toot); err != nil {
		return err
	}
	return nil
}

func (s *Config) cmdMotionSnapshot(status *mastodon.Status) error {

	mresp, err := s.motion.Action("1", "snapshot")
	if err != nil {
		return err
	}

	toot := mastodon.Toot{
		Status:      fmt.Sprintf("@%s\n%s", status.Account.Acct, mresp),
		InReplyToID: status.ID,
		Visibility:  mastodon.VisibilityDirectMessage,
		Language:    "de",
	}
	if _, err := s.Client().PostStatus(s.Ctx(), &toot); err != nil {
		return err
	}
	return nil
}
