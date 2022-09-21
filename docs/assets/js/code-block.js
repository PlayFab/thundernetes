function copyToClipboard() {
    const codeBlockChildNodes = document.getElementById("code-block-text-input").childNodes;
    const codeBlockText = Array.from(codeBlockChildNodes).filter(node => node.innerHTML != undefined).reduce((acc, curr) => acc + curr.innerHTML, "")
    navigator.clipboard.writeText(codeBlockText);
}