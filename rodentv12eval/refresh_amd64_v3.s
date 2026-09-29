//go:build amd64 && amd64.v3

#include "textflag.h"

// The V1.2 refresh is exact modular int16 arithmetic. Each vector lane starts
// from its bias and visits row pointers in the same order as the Go oracle, so
// VPADDW matches it exactly, including overflow.
TEXT ·rodentV12RefreshPerspectiveAVX2(SB), NOSPLIT, $0-32
	MOVQ destination+0(FP), AX
	MOVQ biases+8(FP), BX
	MOVQ rows+16(FP), CX
	MOVQ rowCount+24(FP), DX
	MOVL $48, R8
	XORQ R12, R12
v12_refresh_block:
	VMOVDQU (BX), Y0
	MOVQ CX, R9
	MOVQ DX, R10
v12_refresh_row:
	TESTQ R10, R10
	JZ v12_refresh_store
	MOVQ (R9), R11
	VPADDW (R11)(R12*1), Y0, Y0
	ADDQ $8, R9
	DECQ R10
	JMP v12_refresh_row
v12_refresh_store:
	VMOVDQU Y0, (AX)
	ADDQ $32, AX
	ADDQ $32, BX
	ADDQ $32, R12
	DECL R8
	JNZ v12_refresh_block
	VZEROUPPER
	RET
