package replace_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/ayoisaiah/f2/internal/file"
	"github.com/ayoisaiah/f2/internal/testutil"
)

func getCurrentDate() string {
	now := time.Now()

	year, month, day := now.Date()

	return fmt.Sprintf("%d-%02d-%02d", year, month, day)
}

func TestVariables(t *testing.T) {
	testCases := []testutil.TestCase{
		{
			Name: "replace filename and path variables",
			Changes: []*file.Change{
				{
					BaseDir: "dir3/dir2/dir1",
					Source:  "file.txt",
				},
			},
			Want: []string{
				"dir3/dir2/dir1/dir1_dir3_dir2_file.txt",
			},
			Args: []string{"-f", ".*", "-r", "{p}_{3p}_{2p}_{f}{ext}"},
		},
		{
			Name: "transform string cases",
			Changes: []*file.Change{
				{
					Source: "Ulysses by James Joyce.epub",
				},
			},
			Want: []string{"Ulysses - JAMES JOYCE.Epub"},
			Args: []string{
				"-f",
				"(.*) by (.*)\\.epub",
				"-r",
				"$1 - {<$2>.up}{ext.ti}",
			},
		},
		{
			Name: "remove diacritics",
			Changes: []*file.Change{
				{
					Source: "Café-Übersicht_Été2024.docx",
				},
			},
			Want: []string{"Cafe-Ubersicht_Ete2024.docx"},
			Args: []string{"-f", ".*", "-r", "{.di}"},
		},
		{
			Name: "remove only some diacritics",
			Changes: []*file.Change{
				{
					Source: "Café-Übersicht_Été2024.docx",
				},
			},
			Want: []string{"Cafe-Übersicht-Été2024.docx"},
			Args: []string{
				"-f",
				"(.*)-(.*)_(.*).docx",
				"-r",
				"{<$1>.di}-$2-$3{ext}",
			},
		},
		{
			Name: "parse arbitrary text as date",
			Changes: []*file.Change{
				{
					Source: "Screenshot from 2022-04-12 14:37:35.png",
				},
				{
					Source: "Screenshot from 2022-09-26 21:19:15.png",
				},
			},
			Want: []string{
				"2022/April/Screenshot from 2022-04-12 14:37:35.png",
				"2022/September/Screenshot from 2022-09-26 21:19:15.png",
			},
			Args: []string{
				"-f",
				"Screenshot from (.*)\\.png",
				"-r",
				"{<$1>.dt.YYYY}/{<$1>.dt.MMMM}/{f}{ext}",
			},
		},
		{
			Name: "replace with Exif variables",
			Changes: []*file.Change{
				{
					BaseDir: "testdata",
					Source:  "pic.jpg",
				},
				{
					BaseDir: "testdata",
					Source:  "image.dng",
				},
			},
			Want: []string{
				"testdata/2001_FUJIFILM_FinePix2400Zoom_ISO100_w100_h80_100x80_s_6mm(mm)_f3.5.jpg",
				"testdata/2005_Canon_Canon EOS 350D DIGITAL_ISO400_w8_h8_8x8_1_15s_55mm(mm)_f8.dng",
			},
			Args: []string{
				"-f", ".*", "-r", "{x.cdt.YYYY}_{exif.make}_{exif.model}_ISO{exif.iso}_w{exif.w}_h{exif.h}_{exif.wh}_{exif.et}s_{exif.fl}mm({exif.fl35}mm)_f{x.fnum}{ext}",
			},
		},
		{
			Name: "replace with ID3 variables",
			Changes: []*file.Change{
				{
					BaseDir: "testdata",
					Source:  "audio.flac",
				},
				{
					BaseDir: "testdata",
					Source:  "audio.mp3",
				},
				{
					BaseDir: "testdata",
					Source:  "image.dng",
				},
			},
			Want: []string{
				"testdata/TEST TITLE_Test Artist_VORBIS_FLAC_Test Album_Test AlbumArtist_3_6_2__2000_Jazz_Test Composer.flac",
				"testdata/EXIFTOOL TEST_Phil Harvey_ID3v2.2_MP3_Phil's Greatest Hits__1_5_1_2_2005_Testing_A Composer.mp3",
				"testdata/____________.dng",
			},
			Args: []string{
				"-f", ".*", "-r", "{id3.title.up}_{id3.artist}_{id3.format}_{id3.type}_{id3.album}_{id3.album_artist}_{id3.track}_{id3.total_tracks}_{id3.disc}_{id3.total_discs}_{id3.year}_{id3.genre}_{id3.composer}{ext}",
			},
		},
		{
			Name: "replace with file hash variables",
			Changes: []*file.Change{
				{
					BaseDir: "testdata",
					Source:  "audio.flac",
				},
				{
					BaseDir: "testdata",
					Source:  "pic.jpg",
				},
			},
			Want: []string{
				"testdata/8A37426A720E41D527AEC7E41F483AF7_cdaf50625ba86f59260e7b5b21d1d1446979164a_fbdaadaf82b4c53e434134e2950b185456ab49cf41d64d19b941d139d58daa5a_21aa23e5b70a8f3a3f3264b5539f54b9c60309416f5049f35487da6bc3c4c6b7d4f4f94b91a206950db957886b8377ca7136faaf316b72535dae2c3b32d7bb58",
				"testdata/B760E71C50E07776346524564B263DA1_fcc230bca4f314e486c52dfb658616d3df2413e3_9161967ed308f014d8c8b6c316e844d99dd01a7e0dc9bad3124491bf675e2100_1f02b3427d0e950f45563581c030b376809546a7a5efa9b4b8ec4b5d15e221a3952cac712f8f783f68bd3d6c7032a4ad10abbaddf3596f421cca2bf0f575e67a",
			},
			Args: []string{
				"-f", ".*", "-r", "{hash.md5.up}_{hash.sha1}_{hash.sha256}_{hash.sha512}",
			},
		},
		{
			Name: "replace with Exiftool variables",
			Changes: []*file.Change{
				{
					BaseDir: "testdata",
					Source:  "image.dng",
				},
				{
					BaseDir: "testdata",
					Source:  "pic.jpg",
				},
			},
			Want: []string{
				"testdata/Canon_CANON EOS 350D DIGITAL.dng",
				"testdata/Canon_CANON POWERSHOT A5.jpg",
			},
			Args: []string{
				"-f", ".*", "-r", "{xt.Make}_{{xt.Model.up}}{ext}",
			},
		},
		{
			Name: "use file access and modification times",
			Changes: []*file.Change{
				{
					BaseDir: "testdata",
					Source:  "date.txt",
				},
			},
			Want: []string{
				"testdata/Nov-05-2019.txt",
			}, // date is set in TestMain
			Args: []string{
				"-f",
				".*",
				"-r",
				"{atime.MMM}-{mtime.DD}-{mtime.YYYY}{ext}",
			},
		},
		{
			Name: "use file birth and change times",
			Changes: []*file.Change{
				{
					BaseDir: "testdata",
					Source:  "date.txt",
				},
			},
			Want: []string{
				fmt.Sprintf("testdata/%s.txt", getCurrentDate()),
			},
			Args: []string{
				"-f",
				".*",
				"-r",
				"{btime.YYYY}-{ctime.MM}-{now.DD}{ext}",
			},
		},
	}

	replaceTest(t, testCases)
}
