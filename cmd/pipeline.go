package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"github.com/audiolibrelab/jamcapture/internal/service"
	"strings"
)

func executePipeline(songName string, startStep rune) error {
	if pipeline == "" {
		return nil
	}

	steps := []rune(strings.ToLower(pipeline))

	// Find the starting position in the pipeline
	startIndex := -1
	for i, step := range steps {
		if step == startStep {
			startIndex = i
			break
		}
	}

	if startIndex == -1 {
		return fmt.Errorf("step '%c' not found in pipeline '%s'", startStep, pipeline)
	}

	// Create log writer based on verbose level
	var logWriter io.Writer = io.Discard
	if verboseLevel >= 1 {
		logWriter = os.Stderr
	}

	// Create service instance
	svc := service.New(cfg, cfgFile, logWriter)

	// Execute remaining steps in the pipeline
	for i := startIndex + 1; i < len(steps); i++ {
		step := steps[i]
		fmt.Printf("Pipeline: executing step '%c'...\n", step)

		switch step {
		case 'r':
			if err := svc.StartReady(songName); err != nil {
				return fmt.Errorf("pipeline ready failed: %w", err)
			}

			// Wait for user input to stop recording
			fmt.Println("Pipeline: waiting for sources... Recording will start automatically - Press Enter to stop...")
			scanner := bufio.NewScanner(os.Stdin)
			scanner.Scan()

			if err := svc.StopRecording(); err != nil {
				return fmt.Errorf("pipeline record stop failed: %w", err)
			}
			fmt.Println("Pipeline: recording completed")

		case 'm':
			if err := svc.Mix(songName); err != nil {
				return fmt.Errorf("pipeline mix failed: %w", err)
			}
			fmt.Println("Pipeline: mixing completed")

		case 'p':
			if err := svc.Play(songName); err != nil {
				return fmt.Errorf("pipeline play failed: %w", err)
			}
			fmt.Println("Pipeline: playback completed")

		default:
			return fmt.Errorf("unknown pipeline step: '%c' (valid: r=record, m=mix, p=play)", step)
		}
	}

	return nil
}

func validatePipeline() error {
	if pipeline == "" {
		return nil
	}

	validSteps := map[rune]bool{
		'r': true, // record
		'm': true, // mix
		'p': true, // play
	}

	steps := []rune(strings.ToLower(pipeline))
	for _, step := range steps {
		if !validSteps[step] {
			return fmt.Errorf("invalid pipeline step: '%c' (valid: r=record, m=mix, p=play)", step)
		}
	}

	return nil
}