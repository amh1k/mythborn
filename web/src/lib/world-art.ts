const covers = ['/art/world-watchtower.webp', '/art/world-monastery.webp', '/art/world-crossing.webp']

// A stable illustration per world, independent of its chosen role or saved game state.
export function worldArtwork(id: string) {
  let hash = 2166136261
  for (const character of id) {
    hash ^= character.charCodeAt(0)
    hash = Math.imul(hash, 16777619) >>> 0
  }
  return covers[(hash >>> 16) % covers.length]
}
