package clipboard

import (
	"context"
	"runtime"

	"diffman/internal/util"
)

func CopyText(ctx context.Context, text string) error {
	switch runtime.GOOS {
	case "darwin":
		_, err := util.RunWithStdin(ctx, "", text, "pbcopy")
		return err
	case "linux":
		_, err := util.RunWithStdin(ctx, "", text, "xclip", "-selection", "clipboard")
		return err
	case "windows":
		_, err := util.RunWithStdin(ctx, "", text, "clip")
		return err
	default:
		return nil
	}
}
