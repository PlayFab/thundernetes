window.onload = function(){
    document.querySelectorAll('.code-block-clipboard-btn').forEach(elm =>
        elm.onclick = function (event) {
            const source = event.target;
            let sourceContainer = source.nodeName == "IMG" ?  source.parentNode.parentNode : source.parentNode;
            const targetCodeBlock = sourceContainer.firstChild;
            copyToClipboard(targetCodeBlock);
        }
    );
};

function copyToClipboard(targetCodeBlock) {
    const codeBlockChildNodes = targetCodeBlock.childNodes;
    const codeBlockText = Array.from(codeBlockChildNodes)
        .filter(node => node.innerHTML != undefined)
        .map(node => node.innerHTML.replace("&lt;", "<").replace("&gt;", ">"))
        .reduce((acc, curr) => acc + curr, "");
    navigator.clipboard.writeText(codeBlockText);
}