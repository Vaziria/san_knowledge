// shrinkPhoto makes a picture at most maxSide pixels on its longer side, as
// JPEG, before it is uploaded: a phone photo is often 12 to 50 megapixels,
// and the server keeps 1600 pixels anyway. The browser draws it upright
// (its EXIF orientation); the canvas keeps nothing else of the file, such
// as its location, and the server saves it again besides.
export async function shrinkPhoto(file: Blob, maxSide = 1600): Promise<Uint8Array> {
  const bitmap = await createImageBitmap(file, { imageOrientation: 'from-image' })
  const scale = Math.min(1, maxSide / Math.max(bitmap.width, bitmap.height))
  const canvas = document.createElement('canvas')
  canvas.width = Math.max(1, Math.round(bitmap.width * scale))
  canvas.height = Math.max(1, Math.round(bitmap.height * scale))
  const context = canvas.getContext('2d')
  if (!context) throw new Error('no canvas')
  // See-through parts become white, not black.
  context.fillStyle = '#ffffff'
  context.fillRect(0, 0, canvas.width, canvas.height)
  context.drawImage(bitmap, 0, 0, canvas.width, canvas.height)
  bitmap.close()
  const blob = await new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, 'image/jpeg', 0.85))
  if (!blob) throw new Error('could not make a JPEG')
  return new Uint8Array(await blob.arrayBuffer())
}
