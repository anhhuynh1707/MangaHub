import { toast } from 'sonner'

// Thin wrapper over sonner so the rest of the app has one import for toasts and
// we can tweak defaults/styling in a single place.
export const notify = {
  success: (message: string, description?: string) => toast.success(message, { description }),
  error: (message: string, description?: string) => toast.error(message, { description }),
  info: (message: string, description?: string) => toast(message, { description }),
  warning: (message: string, description?: string) => toast.warning(message, { description }),
}
