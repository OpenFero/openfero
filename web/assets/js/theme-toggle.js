// Immediately apply theme before page load to prevent flash
(function () {
  const getPreferredTheme = () => {
    const storedTheme = localStorage.getItem("theme");
    if (storedTheme) {
      return storedTheme;
    }
    return window.matchMedia("(prefers-color-scheme: dark)").matches
      ? "dark"
      : "light";
  };

  // Apply theme immediately
  document.documentElement.setAttribute("data-bs-theme", getPreferredTheme());
})();

document.addEventListener("DOMContentLoaded", () => {
  // Check for saved theme preference or use OS preference
  const getPreferredTheme = () => {
    const storedTheme = localStorage.getItem("theme");
    if (storedTheme) {
      return storedTheme;
    }
    return window.matchMedia("(prefers-color-scheme: dark)").matches
      ? "dark"
      : "light";
  };

  // Apply theme
  const setTheme = (theme) => {
    document.documentElement.setAttribute("data-bs-theme", theme);
    localStorage.setItem("theme", theme);
    updateToggleButton();
  };

  // Update button icon based on current theme
  const updateToggleButton = () => {
    const currentTheme = document.documentElement.getAttribute("data-bs-theme");
    const themeToggle = document.getElementById("themeToggle");

    if (themeToggle) {
      const moonIcon = themeToggle.querySelector(".theme-toggle-dark-icon");
      const sunIcon = themeToggle.querySelector(".theme-toggle-light-icon");

      if (currentTheme === "dark") {
        moonIcon.style.display = "none";
        sunIcon.style.display = "block";
      } else {
        moonIcon.style.display = "block";
        sunIcon.style.display = "none";
      }
    }
  };

  // Toggle theme
  const toggleTheme = () => {
    const currentTheme = document.documentElement.getAttribute("data-bs-theme");
    setTheme(currentTheme === "dark" ? "light" : "dark");
  };

  // Initialize theme
  setTheme(getPreferredTheme());

  // Add event listener to button if it exists
  const themeToggle = document.getElementById("themeToggle");
  if (themeToggle) {
    themeToggle.addEventListener("click", toggleTheme);
  }

  // Listen for OS theme changes
  window
    .matchMedia("(prefers-color-scheme: dark)")
    .addEventListener("change", () => {
      if (!localStorage.getItem("theme")) {
        setTheme(getPreferredTheme());
      }
    });
});
