import type { FleetNode } from "@/lib/types";

export type FleetServerFilter =
  | "all"
  | "problems"
  | "offline"
  | "dockpilot"
  | "monitored"
  | "billing_due";

function nodePriority(node: FleetNode): number {
  if (node.status === "offline" || node.open_incidents > 0) return 0;
  if (node.status === "warning") return 1;
  if (node.role === "master" || node.connection_type === "local") return 2;
  return 3;
}

export function sortFleetNodes(nodes: FleetNode[]): FleetNode[] {
  return [...nodes].sort((a, b) => {
    const pa = nodePriority(a);
    const pb = nodePriority(b);
    if (pa !== pb) return pa - pb;
    return a.name.localeCompare(b.name);
  });
}

function isBillingDue(node: FleetNode): boolean {
  const due = node.billing?.next_due_date;
  if (!due) return false;
  const dueDate = new Date(due);
  if (Number.isNaN(dueDate.getTime())) return false;
  const days = (dueDate.getTime() - Date.now()) / (1000 * 60 * 60 * 24);
  return days <= 30;
}

function hasProblems(node: FleetNode): boolean {
  return (
    node.status === "offline" ||
    node.status === "warning" ||
    node.open_incidents > 0 ||
    (node.applications?.unhealthy ?? 0) > 0
  );
}

export function filterFleetNodes(
  nodes: FleetNode[],
  filter: FleetServerFilter,
): FleetNode[] {
  switch (filter) {
    case "problems":
      return nodes.filter(hasProblems);
    case "offline":
      return nodes.filter((n) => n.status === "offline");
    case "dockpilot":
      return nodes.filter((n) => n.connection_type === "dockpilot");
    case "monitored":
      return nodes.filter((n) => n.connection_type === "agent");
    case "billing_due":
      return nodes.filter(isBillingDue);
    default:
      return nodes;
  }
}

export function fleetNodeHref(node: FleetNode): string | null {
  if (node.connection_type === "local") return "/sites";
  if (node.connection_type === "dockpilot" && node.base_url) return node.base_url;
  if (node.connection_type === "agent") return `/fleet/servers/${node.id}`;
  return null;
}

export function fleetNodeExternal(node: FleetNode): boolean {
  return node.connection_type === "dockpilot" && !!node.base_url;
}
