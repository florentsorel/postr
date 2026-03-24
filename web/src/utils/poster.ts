export const ALLOWED_POSTER_MIME_TYPES = ["image/jpeg", "image/png", "image/webp"] as const

export function isAllowedPosterMimeType(type: string): boolean {
  return (ALLOWED_POSTER_MIME_TYPES as readonly string[]).includes(type)
}
