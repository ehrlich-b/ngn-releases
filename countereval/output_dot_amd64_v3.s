//go:build amd64 && amd64.v3

#include "textflag.h"

// counterOutputDotAVX2 vectorizes the comparison and independent products,
// then adds only active products in the canonical ascending lane order.
TEXT ·counterOutputDotAVX2(SB), NOSPLIT, $32-20
	MOVQ accumulator+0(FP), AX
	MOVQ weights+8(FP), BX
	MOVQ SP, R8
	VXORPS Y7, Y7, Y7
	VXORPS X6, X6, X6
	MOVL $64, CX
output_block:
	VMOVUPS (AX), Y0
	VCMPPS $30, Y7, Y0, Y2
	VANDPS Y2, Y0, Y1
	VMULPS (BX), Y1, Y1
	VMOVMSKPS Y2, DX
	TESTL DX, DX
	JZ output_next
	VMOVUPS Y1, (R8)
output_active:
	BSFL DX, SI
	VADDSS (R8)(SI*4), X6, X6
	LEAL -1(DX), DI
	ANDL DI, DX
	JNZ output_active
output_next:
	ADDQ $32, AX
	ADDQ $32, BX
	DECL CX
	JNZ output_block
	VMOVSS X6, ret+16(FP)
	VZEROUPPER
	RET
