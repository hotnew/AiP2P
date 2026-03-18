package aip2p

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"
)

const defaultTrackerListINF = `# Trackerlist.inf
# One tracker URI per line. Lines starting with #, ;, or // are ignored.
http://1337.abcvg.info:80/announce
http://bt.okmp3.ru:2710/announce
http://ipv4.rer.lol:2710/announce
http://ipv6.rer.lol:6969/announce
http://lucke.fenesisu.moe:6969/announce
http://nyaa.tracker.wf:7777/announce
http://torrentsmd.com:8080/announce
http://tr.cili001.com:8070/announce
http://tracker.dhitechnical.com:6969/announce
http://tracker.mywaifu.best:6969/announce
http://tracker.renfei.net:8080/announce
http://tracker.skyts.net:6969/announce
http://tracker.waaa.moe:6969/announce
http://tracker.xn--djrq4gl4hvoi.top:80/announce
http://www.all4nothin.net:80/announce.php
http://www.wareztorrent.com:80/announce
https://1337.abcvg.info:443/announce
https://shahidrazi.online:443/announce
https://t.213891.xyz:443/announce
https://torrent.tracker.durukanbal.com:443/announce
https://tr.abiir.top:443/announce
https://tr.abir.ga:443/announce
https://tr.nyacat.pw:443/announce
https://tracker.ghostchu-services.top:443/announce
https://tracker.iochimari.moe:443/announce
https://tracker.kuroy.me:443/announce
https://tracker.manager.v6.navy:443/announce
https://tracker.moeblog.cn:443/announce
https://tracker.novy.vip:443/announce
https://tracker.qingwapt.org:443/announce
https://tracker.zhuqiy.com:443/announce
https://tracker1.520.jp:443/announce
udp://bittorrent-tracker.e-n-c-r-y-p-t.net:1337/announce
udp://bt.rer.lol:6969/announce
udp://d40969.acod.regrucolo.ru:6969/announce
udp://evan.im:6969/announce
udp://extracker.dahrkael.net:6969/announce
udp://martin-gebhardt.eu:25/announce
udp://ns575949.ip-51-222-82.net:6969/announce
udp://open.demonii.com:1337/announce
udp://open.dstud.io:6969/announce
udp://open.stealth.si:80/announce
udp://opentracker.io:6969/announce
udp://p4p.arenabg.com:1337/announce
udp://retracker.lanta.me:2710/announce
udp://t.overflow.biz:6969/announce
udp://torrentvpn.club:6990/announce
udp://tracker-udp.gbitt.info:80/announce
udp://tracker.1h.is:1337/announce
udp://tracker.alaskantf.com:6969/announce
udp://tracker.bittor.pw:1337/announce
udp://tracker.bluefrog.pw:2710/announce
udp://tracker.corpscorp.online:80/announce
udp://tracker.dler.com:6969/announce
udp://tracker.dler.org:6969/announce
udp://tracker.flatuslifir.is:6969/announce
udp://tracker.fnix.net:6969/announce
udp://tracker.gmi.gd:6969/announce
udp://tracker.ixuexi.click:6969/announce
udp://tracker.opentorrent.top:6969/announce
udp://tracker.opentrackr.org:1337/announce
udp://tracker.playground.ru:6969/announce
udp://tracker.qu.ax:6969/announce
udp://tracker.riverarmy.xyz:6969/announce
udp://tracker.skyts.net:6969/announce
udp://tracker.srv00.com:6969/announce
udp://tracker.t-1.org:6969/announce
udp://tracker.theoks.net:6969/announce
udp://tracker.therarbg.to:6969/announce
udp://tracker.torrent.eu.org:451/announce
udp://tracker.torrust-demo.com:6969/announce
udp://tracker.tryhackx.org:6969/announce
udp://tracker.wepzone.net:6969/announce
udp://uabits.today:6990/announce
udp://udp.tracker.projectk.org:23333/announce
udp://wepzone.net:6969/announce
wss://tracker.openwebtorrent.com:443/announce
`

func defaultTrackerListPath(netPath string) string {
	netPath = strings.TrimSpace(netPath)
	if netPath == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(netPath), "Trackerlist.inf")
}

func EnsureDefaultTrackerList(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(defaultTrackerListINF), 0o644)
}

func LoadTrackerList(path string) ([]string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	out := make([]string, 0)
	seen := make(map[string]struct{})
	for _, rawLine := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "//") {
			continue
		}
		if key, value, ok := strings.Cut(line, "="); ok && strings.EqualFold(strings.TrimSpace(key), "tracker") {
			line = strings.TrimSpace(value)
		}
		if line == "" {
			continue
		}
		if _, ok := seen[line]; ok {
			continue
		}
		seen[line] = struct{}{}
		out = append(out, line)
	}
	return out, nil
}

func trackerTiers(trackers []string) [][]string {
	out := make([][]string, 0, len(trackers))
	for _, tracker := range trackers {
		tracker = strings.TrimSpace(tracker)
		if tracker == "" {
			continue
		}
		out = append(out, []string{tracker})
	}
	return out
}

func mergeTrackersIntoSpec(spec *torrent.TorrentSpec, trackers []string) {
	if spec == nil || len(trackers) == 0 {
		return
	}
	seen := make(map[string]struct{})
	merged := make([][]string, 0, len(spec.Trackers)+len(trackers))
	for _, tier := range spec.Trackers {
		if len(tier) == 0 {
			continue
		}
		cleanTier := make([]string, 0, len(tier))
		for _, tracker := range tier {
			tracker = strings.TrimSpace(tracker)
			if tracker == "" {
				continue
			}
			cleanTier = append(cleanTier, tracker)
			seen[tracker] = struct{}{}
		}
		if len(cleanTier) > 0 {
			merged = append(merged, cleanTier)
		}
	}
	for _, tracker := range trackers {
		tracker = strings.TrimSpace(tracker)
		if tracker == "" {
			continue
		}
		if _, ok := seen[tracker]; ok {
			continue
		}
		seen[tracker] = struct{}{}
		merged = append(merged, []string{tracker})
	}
	spec.Trackers = merged
}

func addMagnetWithTrackers(client *torrent.Client, magnet string, trackers []string) (*torrent.Torrent, error) {
	spec, err := torrent.TorrentSpecFromMagnetUri(magnet)
	if err != nil {
		return nil, err
	}
	mergeTrackersIntoSpec(spec, trackers)
	t, _, err := client.AddTorrentSpec(spec)
	return t, err
}

func addTorrentFileWithTrackers(client *torrent.Client, torrentPath string, trackers []string) (*torrent.Torrent, error) {
	mi, err := metainfo.LoadFromFile(torrentPath)
	if err != nil {
		return nil, err
	}
	spec := torrent.TorrentSpecFromMetaInfo(mi)
	mergeTrackersIntoSpec(spec, trackers)
	t, _, err := client.AddTorrentSpec(spec)
	return t, err
}
