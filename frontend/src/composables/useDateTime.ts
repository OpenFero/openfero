import { computed } from 'vue'

export interface DateFormatOptions {
  includeMilliseconds?: boolean
  includeTimezone?: boolean
  dateStyle?: 'full' | 'long' | 'medium' | 'short'
  timeStyle?: 'full' | 'long' | 'medium' | 'short'
}

/**
 * Composable for formatting dates according to the user's browser locale.
 * Uses Intl.DateTimeFormat for proper localization.
 */
export function useDateTime() {
  // Get the user's preferred locale from the browser
  const userLocale = computed(() => {
    return navigator.language || navigator.languages?.[0] || 'en-US'
  })

  /**
   * Format a date/timestamp according to the user's locale
   */
  function formatDateTime(
    dateInput: string | Date | number,
    options: DateFormatOptions = {},
  ): string {
    const { includeMilliseconds = true, includeTimezone = true, dateStyle, timeStyle } = options

    try {
      const date = dateInput instanceof Date ? dateInput : new Date(dateInput)

      // Check for invalid date
      if (Number.isNaN(date.getTime())) {
        return 'Invalid date'
      }

      // Use dateStyle/timeStyle if provided (simpler API)
      if (dateStyle || timeStyle) {
        const formatter = new Intl.DateTimeFormat(userLocale.value, {
          dateStyle,
          timeStyle,
        })
        return formatter.format(date)
      }

      // Default detailed format
      const formatOptions: Intl.DateTimeFormatOptions = {
        year: 'numeric',
        month: 'short',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
        hour12: undefined, // Let the locale decide
      }

      if (includeTimezone) {
        formatOptions.timeZoneName = 'short'
      }

      const formatter = new Intl.DateTimeFormat(userLocale.value, formatOptions)
      let formatted = formatter.format(date)

      // Add milliseconds if requested
      if (includeMilliseconds) {
        const ms = date.getMilliseconds().toString().padStart(3, '0')
        // Insert milliseconds after seconds (before timezone or end)
        formatted = formatted.replace(/(\d{2}:\d{2}:\d{2})/, `$1.${ms}`)
      }

      return formatted
    } catch {
      return String(dateInput)
    }
  }

  /**
   * Format a date for display (date only, no time)
   */
  function formatDate(dateInput: string | Date | number): string {
    return formatDateTime(dateInput, {
      includeMilliseconds: false,
      includeTimezone: false,
      dateStyle: 'medium',
    })
  }

  /**
   * Format a time for display (time only, no date)
   */
  function formatTime(dateInput: string | Date | number, includeSeconds = true): string {
    try {
      const date = dateInput instanceof Date ? dateInput : new Date(dateInput)

      if (Number.isNaN(date.getTime())) {
        return 'Invalid time'
      }

      const options: Intl.DateTimeFormatOptions = {
        hour: '2-digit',
        minute: '2-digit',
        hour12: undefined,
      }

      if (includeSeconds) {
        options.second = '2-digit'
      }

      return new Intl.DateTimeFormat(userLocale.value, options).format(date)
    } catch {
      return String(dateInput)
    }
  }

  /**
   * Get relative time string (e.g., "2 hours ago", "in 3 days")
   */
  function formatRelative(dateInput: string | Date | number): string {
    try {
      const date = dateInput instanceof Date ? dateInput : new Date(dateInput)

      if (Number.isNaN(date.getTime())) {
        return 'Invalid date'
      }

      const now = new Date()
      const diffMs = date.getTime() - now.getTime()
      const diffSeconds = Math.round(diffMs / 1000)
      const diffMinutes = Math.round(diffSeconds / 60)
      const diffHours = Math.round(diffMinutes / 60)
      const diffDays = Math.round(diffHours / 24)

      const rtf = new Intl.RelativeTimeFormat(userLocale.value, {
        numeric: 'auto',
      })

      if (Math.abs(diffSeconds) < 60) {
        return rtf.format(diffSeconds, 'second')
      } else if (Math.abs(diffMinutes) < 60) {
        return rtf.format(diffMinutes, 'minute')
      } else if (Math.abs(diffHours) < 24) {
        return rtf.format(diffHours, 'hour')
      } else {
        return rtf.format(diffDays, 'day')
      }
    } catch {
      return String(dateInput)
    }
  }

  /**
   * Get ISO format string
   */
  function toISOString(dateInput: string | Date | number): string {
    try {
      const date = dateInput instanceof Date ? dateInput : new Date(dateInput)
      return date.toISOString()
    } catch {
      return String(dateInput)
    }
  }

  return {
    userLocale,
    formatDateTime,
    formatDate,
    formatTime,
    formatRelative,
    toISOString,
  }
}
