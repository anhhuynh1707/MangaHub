// Friendly aliases over the auto-generated OpenAPI types in schema.d.ts.
//
// schema.d.ts is produced from the backend Swagger spec by `npm run gen:api`
// (also run automatically as a prebuild step). Do NOT edit schema.d.ts by hand —
// edit the Go annotations and regenerate. Import types from THIS module, not the
// raw `components["schemas"]["models.X"]` paths.
import type { components } from './schema'

type Schemas = components['schemas']

// Request bodies (well-typed in the spec — use these as API input types).
export type LoginRequest = Schemas['models.LoginRequest']
export type RegisterRequest = Schemas['models.RegisterRequest']
export type ChangePasswordRequest = Schemas['models.ChangePasswordRequest']
export type AddToLibraryRequest = Schemas['models.AddToLibraryRequest']
export type UpdateProgressRequest = Schemas['models.UpdateProgressRequest']

// Domain models defined in the spec.
export type MangaSchema = Schemas['models.Manga']

// The standard response envelope, made generic over its data payload.
export type ApiResponse<T = unknown> = Omit<Schemas['utils.APIResponse'], 'data'> & {
  data?: T
}
