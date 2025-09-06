# OpenAntiCheat

OpenAntiCheat is a proof-of-concept attempt at building an anti-cheat.  
The main goal is **learning Windows kernel development** and experimenting with how to detect and act on suspicious process behavior.

## Overview
The system consists of two components:
- **Kernel Driver**  
  Uses ObCallbacks, Notify Routines, and Minifilter events to monitor system activity.  
- **Userland Application**  
  Communicates with the driver via **Minifilter communication ports** (bi-directional).  
  Responsible for scoring processes and taking userland-visible actions.

## Current Detections
- **Thread creation**  
- **Handle creation**  

These are required for nearly every common attack (e.g. DLL injection, remote thread creation), making them good first targets.

## Planned Features
- Proactive checks triggered by **Minifilter events**  
- **Load Image Notify** for DLL / module load detection  
- Expanded scoring rules for additional attack vectors  

## PoC Disclaimer
This project is not production-ready. A real anti-cheat would also need:
- Obfuscation and encryption
- Secure backend persistence
- Hardened communication

These are skipped here since the focus is on prototyping and experimenting with kernel ↔ userland interaction.

## Why?
Anti-cheat development combines kernel security, Windows internals, and attack surface analysis.  
This project is my playground to explore these concepts while building something functional.

---

⚠️ **Disclaimer**: This is a learning project. Do not expect production-grade security.  
