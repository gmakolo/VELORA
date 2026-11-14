const password = document.querySelector("#password");
const toggle = document.querySelector(".visibility-toggle");

toggle.hidden = false;
toggle.addEventListener("click", () => {
    const showPassword = password.type === "password";
    password.type = showPassword ? "text" : "password";
    toggle.textContent = showPassword ? "Hide" : "Show";
});
