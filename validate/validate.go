package validate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/ayoisaiah/f2/internal/config"
	"github.com/ayoisaiah/f2/internal/conflict"
	"github.com/ayoisaiah/f2/internal/file"
	"github.com/ayoisaiah/f2/internal/osutil"
	"github.com/ayoisaiah/f2/internal/pathutil"
	"github.com/ayoisaiah/f2/internal/status"
)

var conflicts conflict.Collection

var changes []*file.Change

const (
	// max filename length of 255 characters in Windows.
	windowsMaxFileCharLength = 255
	// max filename length of 255 bytes on Linux and other unix-based OSes.
	unixMaxBytes = 255
)

// newTarget appends a number to the target file name so that it
// does not conflict with an existing path on the filesystem or
// another renamed file. For example: image.png becomes image(1).png.
func newTarget(change *file.Change) string {
	conf := config.Get()

	counter := 1

	baseName := filepath.Base(change.Target)
	if !change.IsDir {
		baseName = pathutil.StripExtension(baseName)
	}

	regex := conf.FixConflictsPatternRegex

	if regex == nil {
		r := regexp.MustCompile(`%(\d+)?d`)
		regex = regexp.MustCompile(
			r.ReplaceAllString(conf.FixConflictsPattern, `(\d+)`),
		)
	}

	// Extract the numbered index at the end of the filename (if any)
	match := regex.FindStringSubmatch(baseName)

	if len(match) > 0 {
		num, _ := strconv.Atoi(match[1])
		num += counter

		baseName = regex.ReplaceAllString(
			baseName,
			fmt.Sprintf(conf.FixConflictsPattern, num),
		)
	} else {
		baseName += fmt.Sprintf(conf.FixConflictsPattern, counter)
	}

	target := baseName + filepath.Ext(change.Target)

	return filepath.Join(filepath.Dir(change.Target), target)
}

// checkEmptyFilenameConflict reports if the file renaming has resulted
// in an empty string. This conflict is automatically fixed by leaving
// the filename unchanged.
func checkEmptyFilenameConflict(
	change *file.Change,
	autoFix bool,
) (conflictDetected bool) {
	if change.Target == "." || change.Target == "" {
		conflictDetected = true

		if autoFix {
			// The file is left unchanged
			change.Target = change.Source
			change.Status = status.Unchanged

			return
		}

		conflicts[conflict.EmptyFilename] = append(
			conflicts[conflict.EmptyFilename],
			conflict.Conflict{
				Sources: []string{change.RelSourcePath},
				Target:  change.RelTargetPath,
			},
		)
		change.Status = status.EmptyFilename
	}

	return
}

// checkPathExistsConflict reports if the newly renamed path
// already exists on the filesystem.
func checkPathExistsConflict(
	change *file.Change,
	changeIndex int,
	autoFix, allowOverwrites bool,
) (conflictDetected bool) {
	// Report if target path exists on the filesystem
	if _, err := os.Stat(change.RelTargetPath); err == nil ||
		errors.Is(err, os.ErrExist) {
		// Don't report a conflict for an unchanged filename
		if change.RelSourcePath == change.RelTargetPath {
			change.Status = status.Unchanged
			return
		}

		// Case-insensitive filesystems should not report conflicts
		// if only the case of the filename is being changed.
		if strings.EqualFold(change.RelSourcePath, change.RelTargetPath) {
			return
		}

		// Don't report a conflict if overwriting files are allowed
		if allowOverwrites {
			change.WillOverwrite = true
			change.Status = status.Overwriting

			return
		}

		// Don't report a conflict if target path is changing before
		// the source path is renamed
		for i := 0; i < len(changes); i++ {
			ch := changes[i]

			if change.RelTargetPath == ch.RelSourcePath &&
				!strings.EqualFold(ch.RelSourcePath, ch.RelTargetPath) &&
				changeIndex > i {
				return
			}
		}

		conflictDetected = true

		if autoFix {
			change.Target = newTarget(change)
			change.RelTargetPath = filepath.Join(change.BaseDir, change.Target)
			change.Status = status.OK

			return
		}

		conflicts[conflict.FileExists] = append(
			conflicts[conflict.FileExists],
			conflict.Conflict{
				Sources: []string{change.RelSourcePath},
				Target:  change.RelTargetPath,
			},
		)

		change.Status = status.PathExists
	}

	return conflictDetected
}

func checkTargetFileChangingConflict(
	change *file.Change,
	changeIndex int,
	seenPaths map[string]int,
	autoFix bool,
) (conflictDetected bool) {
	var ok bool

	seenIndex, ok := seenPaths[change.RelSourcePath]
	if ok {
		conflictDetected = true
	} else {
		return
	}

	if autoFix {
		changes[seenIndex], changes[changeIndex] = changes[changeIndex], changes[seenIndex]
		return
	}

	conflicts[conflict.TargetFileChanging] = append(
		conflicts[conflict.TargetFileChanging],
		conflict.Conflict{
			Sources: []string{change.RelSourcePath},
			Target:  change.RelTargetPath,
		},
	)

	return
}

// checkOverwritingPathConflict ensures that a newly renamed path
// is not overwritten by another renamed file. Such conflicts are solved by
// appending a number to the filename until no conflict is detected.
func checkOverwritingPathConflict(
	change *file.Change,
	seenPaths map[string]int,
	autoFix bool,
) (conflictDetected bool) {
	if _, ok := seenPaths[change.RelTargetPath]; ok {
		conflictDetected = true
	}

	if !conflictDetected {
		return
	}

	if autoFix {
		change.Target = newTarget(change)
		change.RelTargetPath = filepath.Join(change.BaseDir, change.Target)
		change.Status = status.OK

		return
	}

	conflicts[conflict.OverwritingNewPath] = append(
		conflicts[conflict.OverwritingNewPath],
		conflict.Conflict{
			Sources: []string{change.RelSourcePath},
			Target:  change.RelTargetPath,
		},
	)

	change.Status = status.OverwritingNewPath

	return
}

// checkForbiddenCharacters is responsible for ensuring that target file names
// do not contain forbidden characters for the current OS.
func checkForbiddenCharacters(path string) string {
	if runtime.GOOS == osutil.Windows {
		// partialWindowsForbiddenCharRegex is used here as forward and backward
		// slashes are used for auto creating directories
		if osutil.PartialWindowsForbiddenCharRegex.MatchString(path) {
			return strings.Join(
				osutil.PartialWindowsForbiddenCharRegex.FindAllString(
					path,
					-1,
				),
				",",
			)
		}
	}

	if runtime.GOOS == osutil.Darwin {
		if strings.Contains(path, ":") {
			return ":"
		}
	}

	return ""
}

// isTargetLengthExceeded is responsible for ensuring that the target name length
// does not exceed the maximum value on each supported rating system.
func isTargetLengthExceeded(target string) bool {
	// Get the standalone filename
	filename := filepath.Base(target)

	// max length of 255 characters in windows
	if runtime.GOOS == osutil.Windows &&
		len([]rune(filename)) > windowsMaxFileCharLength {
		return true
	}

	if runtime.GOOS != osutil.Windows &&
		len([]byte(filename)) > unixMaxBytes {
		// max length of 255 bytes on Linux and other unix-based OSes
		return true
	}

	return false
}

// checkTrailingPeriodConflictInWindows reports if the file renaming has
// resulted in files or sub directories that end in trailing dots.
// This conflict is automatically resolved by removing the trailing periods.
func checkTrailingPeriodConflictInWindows(
	change *file.Change,
	autoFix bool,
) (conflictDetected bool) {
	if runtime.GOOS == osutil.Windows {
		pathComponents := strings.Split(change.Target, string(os.PathSeparator))

		for _, v := range pathComponents {
			if v != strings.TrimRight(v, ".") {
				conflictDetected = true

				break
			}
		}

		if autoFix && conflictDetected {
			for j, v := range pathComponents {
				s := strings.TrimRight(v, ".")
				pathComponents[j] = s
			}

			change.Target = strings.Join(
				pathComponents,
				string(os.PathSeparator),
			)
			change.Status = status.OK

			return
		}

		if conflictDetected {
			conflicts[conflict.TrailingPeriod] = append(
				conflicts[conflict.TrailingPeriod],
				conflict.Conflict{
					Sources: []string{change.RelSourcePath},
					Target:  change.RelTargetPath,
				},
			)

			change.Status = status.TrailingPeriod
		}
	}

	return
}

// checkFileNameLengthConflict reports if the file renaming has resulted in a
// name that is longer than the acceptable limit (255 characters in Windows and
// 255 bytes on Unix). This conflict is automatically fixed by removing the
// excess characters/bytes until the name is under the limit.
func checkFileNameLengthConflict(
	change *file.Change,
	autoFix bool,
) (conflictDetected bool) {
	exceeded := isTargetLengthExceeded(change.Target)
	if exceeded {
		conflictDetected = true

		if autoFix {
			if runtime.GOOS == osutil.Windows {
				// trim filename so that it's less than 255 characters
				filename := []rune(filepath.Base(change.Target))
				ext := []rune(filepath.Ext(string(filename)))
				f := []rune(
					pathutil.StripExtension(string(filename)),
				)
				index := windowsMaxFileCharLength - len(ext)
				f = f[:index]
				change.Target = string(f) + string(ext)
			} else {
				// trim filename so that it's no more than 255 bytes
				filename := filepath.Base(change.Target)
				ext := filepath.Ext(filename)
				fileNoExt := pathutil.StripExtension(filename)
				index := unixMaxBytes - len([]byte(ext))

				for {
					if len([]byte(fileNoExt)) > index {
						frune := []rune(fileNoExt)
						fileNoExt = string(frune[:len(frune)-1])

						continue
					}

					break
				}

				change.Target = fileNoExt + ext
				change.Status = status.OK
			}

			return
		}

		cause := "255 bytes"
		if runtime.GOOS == osutil.Windows {
			cause = "255 characters"
		}

		conflicts[conflict.MaxFilenameLengthExceeded] = append(
			conflicts[conflict.MaxFilenameLengthExceeded],
			conflict.Conflict{
				Sources: []string{change.RelSourcePath},
				Target:  change.RelTargetPath,
				Cause:   cause,
			},
		)
		change.Status = status.FilenameLengthExceeded
	}

	return
}

// checkForbiddenCharactersConflict is used to detect if forbidden characters
// are present in the target path for a file or directory according to the
// naming rules of the respective OS. This detection excludes forward and
// backward slashes as their presence has a special meaning in the renaming
// ration (automatic directory creation).
// Conflicts are automatically fixed by removing the culprit characters.
func checkForbiddenCharactersConflict(
	change *file.Change,
	autoFix bool,
) (conflictDetected bool) {
	forbiddenChars := checkForbiddenCharacters(change.Target)
	if forbiddenChars != "" {
		conflictDetected = true

		if autoFix {
			if runtime.GOOS == osutil.Windows {
				change.Target = osutil.PartialWindowsForbiddenCharRegex.ReplaceAllString(
					change.Target,
					"",
				)
			}

			if runtime.GOOS == osutil.Darwin {
				change.Target = strings.ReplaceAll(
					change.Target,
					":",
					"",
				)
			}

			change.Status = status.OK

			return
		}

		conflicts[conflict.InvalidCharacters] = append(
			conflicts[conflict.InvalidCharacters],
			conflict.Conflict{
				Sources: []string{change.RelSourcePath},
				Target:  change.RelTargetPath,
				Cause:   forbiddenChars,
			},
		)

		change.Status = status.InvalidCharacters
	}

	return
}

// detectConflicts checks the renamed files for various conflicts and
// automatically fixes them if allowed.
func detectConflicts(autoFix, allowOverwrites bool) {
	seenPaths := make(map[string]int)

	for i := 0; i < len(changes); i++ {
		change := changes[i]

		detected := checkEmptyFilenameConflict(change, autoFix)
		if detected {
			// no need to check for other conflicts here since the filename
			// is empty. If auto fixed, no renaming will occur for the entry
			continue
		}

		detected = checkTrailingPeriodConflictInWindows(change, autoFix)
		if detected && autoFix {
			// going back an index allows rechecking the path for conflicts once more
			i--
			continue
		}

		detected = checkFileNameLengthConflict(change, autoFix)
		if detected && autoFix {
			i--
			continue
		}

		detected = checkForbiddenCharactersConflict(change, autoFix)
		if detected && autoFix {
			i--
			continue
		}

		detected = checkPathExistsConflict(change, i, autoFix, allowOverwrites)
		if detected && autoFix {
			i--
			continue
		}

		detected = checkOverwritingPathConflict(change, seenPaths, autoFix)
		if detected && autoFix {
			i--
			continue
		}

		detected = checkTargetFileChangingConflict(
			change,
			i,
			seenPaths,
			autoFix,
		)
		if detected && autoFix {
			// start over
			i = -1

			clear(seenPaths)

			continue
		}

		if autoFix {
			change.RelTargetPath = filepath.Join(change.BaseDir, change.Target)
		}

		if _, ok := seenPaths[change.RelTargetPath]; !ok {
			seenPaths[change.RelTargetPath] = i
		}
	}
}

// Validate detects and reports any conflicts that can occur while renaming a
// file. Conflicts are automatically fixed if specified in the program options.
func Validate(
	matches []*file.Change,
	autoFix, allowOverwrites bool,
) conflict.Collection {
	conflicts = make(conflict.Collection)

	changes = matches

	detectConflicts(autoFix, allowOverwrites)

	return conflicts
}

func GetConflicts() conflict.Collection {
	return conflicts
}
