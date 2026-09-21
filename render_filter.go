package main

// WebGPU exposes standard anisotropy levels up to 16. Sampler changes are
// applied by the renderer at the start of its next frame.
func supportedAnisotropy() int            { return 16 }
func atlasSourceCoordinate(pixel int) int { return max(0, min(atlasTileSize-1, pixel-atlasPadding)) }
