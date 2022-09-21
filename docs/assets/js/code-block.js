function copyToClipboard() {
    codeBlockText = document.getElementById("code-block-text-input").value;
    navigator.clipboard.writeText(codeBlockText);
}