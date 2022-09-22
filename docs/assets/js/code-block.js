window.onload = function(){
    document.querySelectorAll('.code-block-clipboard-btn').forEach(elm =>
        elm.onclick = function (event) {
            const sourceBtn = event.target.nodeName == "IMG" ? event.target.parentNode : event.target;
            let sourceContainer = sourceBtn.parentNode;
            const targetCodeBlock = sourceContainer.firstChild;
            copyToClipboard(targetCodeBlock);
            doCheckAnimation(sourceBtn);
        }
    );
};

function doCheckAnimation(sourceBtn) {
    let icon = sourceBtn.childNodes[1];
    icon.src = "/thundernetes/assets/images/check-solid.svg";
    setTimeout(function(){
        icon.src = "/thundernetes/assets/images/copy-regular.svg";
    }, 1000);
}

function copyToClipboard(targetCodeBlock) {
    const codeBlockChildNodes = targetCodeBlock.childNodes;
    const codeBlockText = Array.from(codeBlockChildNodes)
        .filter(node => node.innerHTML != undefined)
        .map(node => node.innerHTML.replace("&lt;", "<").replace("&gt;", ">"))
        .reduce((acc, curr) => acc + curr, "");
    navigator.clipboard.writeText(codeBlockText);
}