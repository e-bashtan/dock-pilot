import type { ServerNode } from "@/lib/types";

export type ServerFilter =
  | "all"
  | "problems"
  | "offline"
  | "barn"
  | "monitored"
  | "billing_due";

/** Full panel node (Barn), including legacy API value "dockpilot". */
export function isBarnPanel(node: Pick<ServerNode, "connection_type">): boolean {
  return node.connection_type === "barn" || node.connection_type === "dockpilot";
}

function nodePriority(node: ServerNode): number {
  if (node.status === "offline" || node.open_incidents > 0) return 0;
  if (node.status === "warning") return 1;
  if (node.role === "master" || node.connection_type === "local") return 2;
  return 3;
}

export function sortServerNodes(nodes: ServerNode[]): ServerNode[] {
  return [...nodes].sort((a, b) => {
    const pa = nodePriority(a);
    const pb = nodePriority(b);
    if (pa !== pb) return pa - pb;
    return a.name.localeCompare(b.name);
  });
}

function isBillingDue(node: ServerNode): boolean {
  const days = node.billing?.days_left;
  if (typeof days === "number") {
    const alert = node.billing?.alert_days && node.billing.alert_days > 0
      ? node.billing.alert_days
      : 10;
    return days <= alert;
  }
  const due = node.billing?.next_due_date;
  if (!due) return false;
  const dueDate = new Date(due);
  if (Number.isNaN(dueDate.getTime())) return false;
  const left = (dueDate.getTime() - Date.now()) / (1000 * 60 * 60 * 24);
  return left <= 10;
}

function hasProblems(node: ServerNode): boolean {
  return (
    node.status === "offline" ||
    node.status === "warning" ||
    node.open_incidents > 0 ||
    (node.applications?.unhealthy ?? 0) > 0
  );
}

export function filterServerNodes(
  nodes: ServerNode[],
  filter: ServerFilter,
): ServerNode[] {
  switch (filter) {
    case "problems":
      return nodes.filter(hasProblems);
    case "offline":
      return nodes.filter((n) => n.status === "offline");
    case "barn":
      return nodes.filter(isBarnPanel);
    case "monitored":
      return nodes.filter((n) => n.connection_type === "agent");
    case "billing_due":
      return nodes.filter(isBillingDue);
    default:
      return nodes;
  }
}

export function serverNodeHref(node: ServerNode): string | null {
  if (node.connection_type === "local") return "/sites";
  if (isBarnPanel(node) && node.base_url) return node.base_url;
  if (node.connection_type === "agent") return `/servers/${node.id}`;
  return null;
}

export function serverNodeExternal(node: ServerNode): boolean {
  return isBarnPanel(node) && !!node.base_url;
}
