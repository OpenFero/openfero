/**
 * Build information for the application
 */
export interface BuildInfo {
  /** Application version */
  version: string
  /** Git commit hash */
  commit: string
  /** Build date */
  buildDate: string
}
