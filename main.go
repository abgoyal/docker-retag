package main

import (
	"fmt"
	"os"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/spf13/cobra"
	"github.com/xeonx/timeago"
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "docker-retag <source-image> <new-tag>",
		Short: "An idempotent tool to point a remote container tag at a new source image.",
		Long: `docker-retag efficiently updates a remote tag (e.g., :prod, :staging) to point
to the manifest of a new source image (e.g., :build-12345).

It is designed for CI/CD pipelines:
- It will overwrite the destination tag if it exists.
- It is idempotent: if the tag already points to the correct image,
  it reports success and does nothing.
- It provides rich output, including image creation timestamps for auditing.`,
		Args: cobra.ExactArgs(2),
		Run:  retagImage,
	}

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// core
func retagImage(cmd *cobra.Command, args []string) {
	sourceImageStr := args[0]
	newTag := args[1]

	sourceRef, err := name.ParseReference(sourceImageStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[FAIL] Error: Invalid source image reference '%s': %v\n", sourceImageStr, err)
		os.Exit(1)
	}
	newRef := sourceRef.Context().Tag(newTag)

	// Step 1: Get the full metadata for the source image. This MUST succeed.
	sourceImg, err := remote.Image(sourceRef, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		fmt.Fprintf(os.Stderr, "[FAIL] Error: Source image '%s' not found or inaccessible: %v\n", sourceImageStr, err)
		os.Exit(1)
	}
	sourceDigest, sourceTimestamp := getImageDetails(sourceImg)

	// Step 2: Get metadata for the destination tag. This may or may not exist.
	destImg, err := remote.Image(newRef, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	var destDigest v1.Hash
	var destTimestamp time.Time
	if err == nil {
		destDigest, destTimestamp = getImageDetails(destImg)
	}

	// Step 3: Check for idempotency.
	if err == nil && sourceDigest.String() == destDigest.String() {
		fmt.Printf("[OK] Tag '%s' already points to the correct image (digest %s, created %s). No action needed.\n", newTag, sourceDigest.String(), formatTime(sourceTimestamp))
		return
	}

	// Step 4: Perform the tag operation. This will create or overwrite the tag.
	if err := crane.Tag(sourceImageStr, newTag, crane.WithAuthFromKeychain(authn.DefaultKeychain)); err != nil {
		fmt.Fprintf(os.Stderr, "[FAIL] Error: Failed to point tag '%s' to new image: %v\n", newTag, err)
		os.Exit(1)
	}

	// Step 5: Final, message.
	fromMsg := ""
	if err == nil {
		fromMsg = fmt.Sprintf(" (was %s, created %s)", destDigest.String(), formatTime(destTimestamp))
	}
	fmt.Printf("[OK] Successfully pointed tag '%s' to %s (created %s)%s.\n", newTag, sourceDigest.String(), formatTime(sourceTimestamp), fromMsg)
}

// extract the digest and creation timestamp
func getImageDetails(img v1.Image) (v1.Hash, time.Time) {
	digest, _ := img.Digest()
	configFile, _ := img.ConfigFile()
	return digest, configFile.Created.Time
}

// human-friendly time string
func formatTime(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	return timeago.English.Format(t)
}
