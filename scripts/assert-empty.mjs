let content = "";
process.stdin.setEncoding("utf8");
for await (const chunk of process.stdin) content += chunk;
if (content.trim().length > 0) {
  console.error(content.trim());
  process.exit(1);
}
