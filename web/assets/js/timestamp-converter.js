// Function to convert timestamps to local or specified timezone
function convertTimestampsToLocalTimezone() {
    document.querySelectorAll('.server-timestamp').forEach(function (element) {
        const serverTimestamp = element.getAttribute('data-timestamp')
        const targetTimezone =
            element.getAttribute('data-timezone') ||
            Intl.DateTimeFormat().resolvedOptions().timeZone
        const showMilliseconds =
            element.hasAttribute('data-precision') &&
            element.getAttribute('data-precision') === 'ms'

        if (serverTimestamp) {
            try {
                let date = null

                // Handle Go format with timezone (most complex case)
                // Examples: "2023-06-15 08:30:00 +0000 UTC", "2023-06-15 08:30:00.12345 +0000 UTC"
                const goFormatMatch = serverTimestamp.match(
                    /^(\d{4}-\d{2}-\d{2})\s(\d{2}:\d{2}:\d{2})(?:\.(\d+))?\s([+-]\d{4})\s\w+/
                )
                if (goFormatMatch) {
                    const [, datePart, timePart, msPart, offset] = goFormatMatch
                    // Ensure we preserve milliseconds precision in the ISO string
                    const isoString = `${datePart}T${timePart}${msPart ? '.' + msPart : ''}${offset}`
                    date = new Date(isoString)
                } // Try standard formats
                else {
                    date = new Date(serverTimestamp)
                }

                if (date && !isNaN(date.getTime())) {
                    // Format options
                    const options = {
                        dateStyle: 'medium',
                        timeStyle: 'medium',
                        timeZone: targetTimezone
                    }

                    // Add milliseconds if requested
                    if (showMilliseconds) {
                        // We need to use individual components for millisecond support
                        delete options.dateStyle
                        delete options.timeStyle

                        Object.assign(options, {
                            year: 'numeric',
                            month: 'short',
                            day: 'numeric',
                            hour: 'numeric',
                            minute: 'numeric',
                            second: 'numeric',
                            fractionalSecondDigits: 3 // For milliseconds
                        })

                        // Add a class for additional styling if desired
                        element.classList.add('with-milliseconds')
                    }

                    element.textContent = new Intl.DateTimeFormat(
                        navigator.language,
                        options
                    ).format(date)

                    // Store original timestamp and converted time for potential tooltips
                    element.setAttribute('title', `Original: ${serverTimestamp}`)

                    return
                }

                console.warn('Could not parse date: ', serverTimestamp)
                element.textContent = serverTimestamp // Fall back to original
            } catch (e) {
                console.error('Error parsing date: ', e)
                element.textContent = serverTimestamp // Fall back to original
            }
        }
    })
}

// Run converter when DOM is loaded
document.addEventListener('DOMContentLoaded', convertTimestampsToLocalTimezone)

// Also run after HTMX content is loaded
document.addEventListener('htmx:afterSwap', convertTimestampsToLocalTimezone)
