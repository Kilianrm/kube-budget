export function buildManifestApplyCommand(fileName: string, context?: string, namespace = "default", kubeconfigPath?: string): string | null {
  if (!fileName.trim() || !context?.trim()) return null;
  if ([fileName, context, namespace, kubeconfigPath ?? ""].some((value) => /[\0\r\n]/.test(value))) return null;

  const quote = (value: string) => `'${value.replace(/'/g, "'\\''")}'`;
  const args = ["kubectl", `--context=${quote(context)}`, `--namespace=${quote(namespace || "default")}`];
  if (kubeconfigPath) args.push(`--kubeconfig=${quote(kubeconfigPath)}`);
  args.push("apply", "-f", quote(fileName.startsWith("-") ? `./${fileName}` : fileName));
  return args.join(" ");
}