import http from "k6/http";
import { check, sleep } from "k6";
import { Rate } from "k6/metrics";

const baseURL = __ENV.BASE_URL || "http://localhost:8080";
const failures = new Rate("task_request_failures");

export const options = {
  stages: [
    { duration: "15s", target: 10 },
    { duration: "30s", target: 30 },
    { duration: "15s", target: 0 },
  ],
  thresholds: {
    http_req_duration: ["p(95)<500"],
    http_req_failed: ["rate<0.01"],
    task_request_failures: ["rate<0.01"],
  },
};

const jsonHeaders = { headers: { "Content-Type": "application/json" } };

function record(response, expectedStatus, label) {
  const ok = check(response, {
    [`${label}: status ${expectedStatus}`]: (res) => res.status === expectedStatus,
  });
  failures.add(!ok);
  return ok;
}

function createTask(assignee, suffix) {
  const response = http.post(
    `${baseURL}/api/v1/tasks`,
    JSON.stringify({
      title: `k6 task ${suffix}`,
      description: "created by the k6 mixed workload",
      status: "pending",
      assignee,
    }),
    jsonHeaders,
  );

  if (!record(response, 201, "create")) {
    return 0;
  }

  return response.json("task.id");
}

export function setup() {
  const assignee = `k6-${Date.now()}`;
  const ids = [];

  for (let index = 0; index < 50; index += 1) {
    const id = createTask(assignee, `seed-${index}`);
    if (id) {
      ids.push(id);
    }
  }

  if (ids.length === 0) {
    throw new Error("could not seed any tasks");
  }

  return { assignee, ids };
}

export default function (data) {
  const listResponse = http.get(`${baseURL}/api/v1/tasks?limit=20`);
  record(listResponse, 200, "list");

  const choice = Math.random();
  const id = data.ids[Math.floor(Math.random() * data.ids.length)];

  if (choice < 0.45) {
    const filtered = http.get(
      `${baseURL}/api/v1/tasks?assignee=${encodeURIComponent(data.assignee)}&limit=20`,
    );
    record(filtered, 200, "filtered list");
  } else if (choice < 0.7) {
    const getResponse = http.get(`${baseURL}/api/v1/tasks/${id}`);
    record(getResponse, 200, "get");
  } else if (choice < 0.85) {
    createTask(data.assignee, `${__VU}-${__ITER}`);
  } else {
    const createdID = createTask(data.assignee, `lifecycle-${__VU}-${__ITER}`);
    if (createdID) {
      const patchResponse = http.patch(
        `${baseURL}/api/v1/tasks/${createdID}`,
        JSON.stringify({ status: "in_progress" }),
        jsonHeaders,
      );
      record(patchResponse, 200, "patch");

      const deleteResponse = http.del(`${baseURL}/api/v1/tasks/${createdID}`);
      record(deleteResponse, 204, "delete");
    }
  }

  sleep(0.2);
}

export function teardown(data) {
  let cursor = 0;

  do {
    const query = `assignee=${encodeURIComponent(data.assignee)}&limit=50${cursor ? `&cursor=${cursor}` : ""}`;
    const response = http.get(`${baseURL}/api/v1/tasks?${query}`);
    if (!record(response, 200, "cleanup list")) {
      return;
    }

    const body = response.json();
    for (const task of body.tasks) {
      record(http.del(`${baseURL}/api/v1/tasks/${task.id}`), 204, "cleanup delete");
    }
    cursor = body.has_more ? body.next_cursor : 0;
  } while (cursor);
}
