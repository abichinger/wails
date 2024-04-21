// Cynhyrchwyd y ffeil hon yn awtomatig. PEIDIWCH Â MODIWL
// This file is automatically generated. DO NOT EDIT

import {Call} from "/wails/runtime.js";

import * as debug from "../runtime/debug/models";
import * as uuid from "../github.com/google/uuid/models";

export function BuildInfo(): Promise<[debug.BuildInfo | null, boolean]> {
    return Call.ByName("main.GreetService.BuildInfo");
}

export function Greet(name: string): Promise<string> {
    return Call.ByName("main.GreetService.Greet", name);
}

export function UUID(): Promise<uuid.UUID> {
    return Call.ByName("main.GreetService.UUID");
}
