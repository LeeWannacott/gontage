package gontage

import (
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io/fs"
	"log"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

func Gontage(sprite_source_folder string, hframes *int) {
	start := time.Now()
	pwd, err := os.Getwd()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	sprites_folder, err := os.ReadDir(filepath.Join(pwd, sprite_source_folder))
	if err != nil {
		log.Fatal(err)
	} else if len(sprites_folder) == 0 {
		log.Fatalf("Looks like folder %v is empty...", sprite_source_folder)
	}

	var chunkSize int
	if runtime.NumCPU() > 12 && runtime.NumCPU()%4 == 0 {
		chunkSize = runtime.NumCPU() / 4
	} else {
		chunkSize = 6
	}

	var chunk_images_waitgroup sync.WaitGroup
	all_decoded_images := make([]image.Image, len(sprites_folder))
	for i := 0; i < len(sprites_folder); i += chunkSize {
		start := i
		end := start + chunkSize
		if end > len(sprites_folder) {
			end = len(sprites_folder)
		}

		chunk_images_waitgroup.Add(1)
		go func(start int, end int) {
			// Ideally decodeImages would write into all_decoded_images directly.
			one_chunk_of_decoded_images := decodeImages(sprites_folder[start:end], sprite_source_folder, pwd, &chunk_images_waitgroup)
			for j, decoded_image := range one_chunk_of_decoded_images {
				all_decoded_images[start+j] = decoded_image
			}
		}(start, end)
	}
	chunk_images_waitgroup.Wait()

	spritesheet_width, spritesheet_height, vframes := calcSpritesheetDimensions(*hframes, all_decoded_images)

	spritesheet := image.NewNRGBA(image.Rect(0, 0, spritesheet_width, spritesheet_height))
	draw.Draw(spritesheet, spritesheet.Bounds(), spritesheet, image.Point{}, draw.Src)
	decoded_images_to_draw_chunked := sliceChunk(all_decoded_images, *hframes)

	var make_spritesheet_wg sync.WaitGroup
	for count_vertical_frames, sprite_chunk := range decoded_images_to_draw_chunked {
		make_spritesheet_wg.Add(1)
		go func(count_vertical_frames int, sprite_chunk []image.Image) {
			defer make_spritesheet_wg.Done()
			drawSpritesheet(sprite_chunk, *hframes, int(vframes), count_vertical_frames, spritesheet)
		}(count_vertical_frames, sprite_chunk)
	}
	make_spritesheet_wg.Wait()
	spritesheet_name := fmt.Sprintf("%v_f%v_v%v.png", sprite_source_folder, len(all_decoded_images), vframes)
	f, err := os.Create(spritesheet_name)
	if err != nil {
		panic(err)
	}
	encoder := png.Encoder{CompressionLevel: png.BestSpeed}
	if err = encoder.Encode(f, spritesheet); err != nil {
		log.Printf("failed to encode: %v", err)
	}
	f.Close()
	fmt.Println("g", time.Since(start))
}

func decodeImages(sprites_folder []fs.DirEntry, targetFolder string, pwd string, wg *sync.WaitGroup) []image.Image {
	defer wg.Done()
	var sprites_array []image.Image
	for _, sprite := range sprites_folder {
		if reader, err := os.Open(filepath.Join(pwd, targetFolder, sprite.Name())); err == nil {
			m, _, err := image.Decode(reader)
			if err != nil {
				log.Fatal(err)
			}
			sprites_array = append(sprites_array, m)
			reader.Close()
		}
	}
	return sprites_array
}

func drawSpritesheet(sprites []image.Image, hframes int, vframes int, count_vertical_frames int, spritesheet draw.Image) {
	for count_horizontal_frames, sprite_image := range sprites {
		bounds := sprite_image.Bounds()
		width := bounds.Dx()
		height := bounds.Dy()
		draw.Draw(spritesheet, image.Rect(count_horizontal_frames*height, count_vertical_frames*width, width*hframes, height*vframes), sprite_image, image.Point{}, draw.Over)
	}
}

func sliceChunk[T any](slice []T, chunkSize int) [][]T {
	var chunks [][]T
	for i := 0; i < len(slice); i += chunkSize {
		end := i + chunkSize
		if end > len(slice) {
			end = len(slice)
		}
		chunks = append(chunks, slice[i:end])
	}
	return chunks
}

func calcSpritesheetDimensions(hframes int, all_decoded_images []image.Image) (int, int, float64) {
	vframes := math.Ceil(float64(len(all_decoded_images)/hframes) + 1)
	var spritesheet_width int
	var spritesheet_height int
	for _, image := range all_decoded_images[:hframes] {
		spritesheet_width += image.Bounds().Dx()
	}
	for _, image := range all_decoded_images[:int(vframes)] {
		spritesheet_height += image.Bounds().Dy()
	}
	return spritesheet_width, spritesheet_height, vframes
}