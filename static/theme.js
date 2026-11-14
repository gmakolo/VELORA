(() => {
    const key = "velora-theme";
    const root = document.documentElement;
    try {
        root.dataset.theme = localStorage.getItem(key) === "dark" ? "dark" : "light";
    } catch {
        root.dataset.theme = "light";
    }

    document.addEventListener("DOMContentLoaded", () => {
        const toggle = document.querySelector("#dark-mode");
        if (!toggle) return;
        toggle.checked = root.dataset.theme === "dark";
        toggle.addEventListener("change", () => {
            const theme = toggle.checked ? "dark" : "light";
            root.dataset.theme = theme;
            try {
                localStorage.setItem(key, theme);
            } catch {
                document.querySelector("#theme-notice").hidden = false;
            }
        });
    });
})();
