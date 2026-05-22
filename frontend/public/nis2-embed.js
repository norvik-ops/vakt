/**
 * Vakt NIS2 Embed Helper — postMessage-based iframe auto-resize.
 * Usage: <script src="https://your-vakt-instance/nis2-embed.js"></script>
 *        <iframe id="nis2" src="https://your-vakt-instance/nis2-check"></iframe>
 *
 * The iframe posts { type: "nis2:resize", height: <px> } on every layout change.
 * This script listens from the parent page and adjusts the iframe height.
 */
(function () {
  "use strict";
  function onMessage(event) {
    if (!event.data || event.data.type !== "nis2:resize") return;
    var frames = document.querySelectorAll("iframe");
    for (var i = 0; i < frames.length; i++) {
      try {
        if (frames[i].contentWindow === event.source) {
          frames[i].style.height = event.data.height + "px";
          break;
        }
      } catch (_) {}
    }
  }
  if (window.addEventListener) {
    window.addEventListener("message", onMessage, false);
  }
})();
